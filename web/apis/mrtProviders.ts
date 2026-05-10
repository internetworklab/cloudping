import { defaultRouteQueryType, RouteQueryType } from "./types";
import mrtDataFromRirAfrinic from "../public/example_mrt_entries_rir-afrinic.json";
import mrtDataFromRirApnic from "../public/example_mrt_entries_rir-apnic.json";
import mrtDataFromRirArin from "../public/example_mrt_entries_rir-arin.json";
import mrtDataFromRirLacnic from "../public/example_mrt_entries_rir-lacnic.json";
import mrtDataFromRirRipencc from "../public/example_mrt_entries_rir-ripencc.json";
import { parse as parseIP, parseCIDR, isValidCIDR, isValid } from "ipaddr.js";
import { useState, useEffect, useCallback, useRef } from "react";
import { LineTokenizer, JSONLineDecoder, getApiEndpoint } from "./globalping";

export function getMRTEntryServiceAPIPrefix(): string {
  return (
    process.env["NEXT_PUBLIC_ROUTE_INFO_API_PREFIX"] ||
    getApiEndpoint() + "/proxy/mrt"
  );
}

export enum ProviderStatus {
  Provisioning = "provisioning",
  Ready = "ready",
}

export interface MRTEntriesProvider {
  Name: string;
  Status: ProviderStatus;
  LastModified: string;
}

export interface MRTEntriesServerResponse<T = unknown> {
  data?: T;
  error?: string;
}

export class DBMRTEntryProvidersLister implements MRTEntryProvidersLister {
  public constructor(public readonly apiPrefix: string) {}

  private buildURL(apiPrefix: string): string {
    const url = `${apiPrefix}/providers`;
    return url;
  }

  public getMRTEntriesProviders(): Promise<
    MRTEntriesServerResponse<MRTEntriesProvider[]>
  > {
    const url = this.buildURL(this.apiPrefix);
    return fetch(url).then((r) => r.json());
  }
}

export interface MRTEntryProvidersLister {
  getMRTEntriesProviders(): Promise<
    MRTEntriesServerResponse<MRTEntriesProvider[]>
  >;
}

export class MockedMRTEntryProvidersLister implements MRTEntryProvidersLister {
  public async getMRTEntriesProviders(): Promise<
    MRTEntriesServerResponse<MRTEntriesProvider[]>
  > {
    return {
      data: [
        {
          Name: "rir-ripencc",
          Status: ProviderStatus.Ready,
          LastModified: "2025-06-20T12:00:00Z",
        },
        {
          Name: "rir-arin",
          Status: ProviderStatus.Ready,
          LastModified: "2025-06-19T08:30:00Z",
        },
        {
          Name: "rir-afrinic",
          Status: ProviderStatus.Provisioning,
          LastModified: "2025-06-18T15:45:00Z",
        },
        {
          Name: "rir-lacnic",
          Status: ProviderStatus.Ready,
          LastModified: "2025-06-17T22:10:00Z",
        },
        {
          Name: "rir-apnic",
          Status: ProviderStatus.Provisioning,
          LastModified: "2025-06-16T06:00:00Z",
        },
      ],
    };
  }
}

export interface MRTEntry {
  Prefix: string;
  Peer?: string;
  PeerAS?: number;
  ASPath?: number[];
  Communities?: number[];
  LargeCommunities?: [number, number, number][];
  ExtendedCommunities?: number[];
  NextHop?: string;
}

export interface MRTEntryDataEvent {
  Data?: MRTEntry;
  Err?: string;
}

/** Wire format for the backend's paginated ndjson stream. */
export interface ResumableResponseStreamEvent {
  data: MRTEntryDataEvent;
  cursor_id?: string;
}

export interface MRTEntriesLister {
  // Returns an ndjson stream, where each line is a JSON-encoded ResumableResponseStreamEvent.
  getMRTEntries(
    abortController: AbortController,
    provider: string,
    queryType: RouteQueryType,
    target: string,
  ): Promise<ReadableStream>;

  // Resume a paginated stream from the given cursor.
  getMRTEntriesByCursor(
    abortController: AbortController,
    provider: string,
    cursorId: string,
  ): Promise<ReadableStream>;
}

export class MockedMRTEntriesLister implements MRTEntriesLister {
  private filterData(
    allData: MRTEntryDataEvent[],
    queryType: RouteQueryType,
    target: string,
  ): Promise<MRTEntryDataEvent[]> {
    switch (queryType) {
      case RouteQueryType.IP_Or_CIDR:
        return this.filterByIPorCIDR(allData, target);
      case RouteQueryType.AS_Path_Segs:
        return this.filterByASPath(allData, target);
      case RouteQueryType.Neighbor_ASN:
        return this.filterByNeighborASN(allData, target);
      case RouteQueryType.Origin_ASN:
        return this.filterByOriginASN(allData, target);
      default:
        return Promise.reject(new Error("unknown query type"));
    }
  }

  private filterByIPorCIDR(
    allData: MRTEntryDataEvent[],
    target: string,
  ): Promise<MRTEntryDataEvent[]> {
    let targetIP: ReturnType<typeof parseIP> | undefined;
    let targetRange: ReturnType<typeof parseCIDR> | undefined;

    if (isValid(target)) {
      targetIP = parseIP(target);
    } else if (isValidCIDR(target)) {
      targetRange = parseCIDR(target);
    } else {
      return Promise.reject(new Error(`invalid IP or CIDR: ${target}`));
    }

    const filtered = allData.filter((entry) => {
      const prefix = entry.Data?.Prefix ?? "";
      if (!isValidCIDR(prefix)) {
        return false;
      }
      const entryRange = parseCIDR(prefix);

      if (targetIP) {
        // entry prefix includes the target IP
        return (
          entryRange.length > 0 &&
          entryRange[0].kind() === targetIP.kind() &&
          targetIP.match(entryRange)
        );
      }

      // target is CIDR: entry prefix covers the target CIDR
      // (entry is a supernet — prefix len <= target prefix len — and contains the target base IP)
      const [targetAddr, targetBits] = targetRange!;
      const [, entryBits] = entryRange;
      return (
        entryBits <= targetBits &&
        entryRange.length > 0 &&
        entryRange[0].kind() === targetAddr.kind() &&
        targetAddr.match(entryRange)
      );
    });

    return Promise.resolve(filtered);
  }

  private parseTargetASSet(target: string): Set<number> {
    const parts = target.split(",").map((s) => s.trim());
    const asnSet = new Set<number>();
    for (const part of parts) {
      if (part === "") {
        throw new Error(`invalid ASN: ${target}`);
      }
      const asn = parseInt(part, 10);
      if (isNaN(asn)) {
        throw new Error(`invalid ASN: ${target}`);
      }
      asnSet.add(asn);
    }
    if (asnSet.size === 0) {
      throw new Error(`invalid ASN: ${target}`);
    }
    return asnSet;
  }

  private filterByASPath(
    allData: MRTEntryDataEvent[],
    target: string,
  ): Promise<MRTEntryDataEvent[]> {
    let targetASNs: Set<number>;
    try {
      targetASNs = this.parseTargetASSet(target);
    } catch (e) {
      return Promise.reject(e);
    }

    // Return all entries whose AS path is a superset of the target ASN set
    return Promise.resolve(
      allData.filter((entry) => {
        const path = entry.Data?.ASPath;
        if (!path) return false;
        const pathSet = new Set(path);
        for (const asn of targetASNs) {
          if (!pathSet.has(asn)) return false;
        }
        return true;
      }),
    );
  }

  private filterByNeighborASN(
    allData: MRTEntryDataEvent[],
    target: string,
  ): Promise<MRTEntryDataEvent[]> {
    const asn = parseInt(target, 10);
    if (isNaN(asn)) {
      return Promise.reject(new Error(`invalid ASN: ${target}`));
    }
    return Promise.resolve(
      allData.filter(
        (entry) =>
          entry.Data?.PeerAS === asn ||
          (Array.isArray(entry.Data?.ASPath) &&
            entry.Data.ASPath.length > 0 &&
            entry.Data.ASPath[0] === asn),
      ),
    );
  }

  private filterByOriginASN(
    allData: MRTEntryDataEvent[],
    target: string,
  ): Promise<MRTEntryDataEvent[]> {
    const asn = parseInt(target, 10);
    if (isNaN(asn)) {
      return Promise.reject(new Error(`invalid ASN: ${target}`));
    }
    return Promise.resolve(
      allData.filter(
        (entry) =>
          entry.Data?.ASPath &&
          entry.Data.ASPath[entry.Data.ASPath.length - 1] === asn,
      ),
    );
  }

  private dataForProvider(provider: string): MRTEntryDataEvent[] {
    switch (provider) {
      case "rir-afrinic":
        return mrtDataFromRirAfrinic as unknown as MRTEntryDataEvent[];
      case "rir-apnic":
        return mrtDataFromRirApnic as unknown as MRTEntryDataEvent[];
      case "rir-arin":
        return mrtDataFromRirArin as unknown as MRTEntryDataEvent[];
      case "rir-lacnic":
        return mrtDataFromRirLacnic as unknown as MRTEntryDataEvent[];
      case "rir-ripencc":
        return mrtDataFromRirRipencc as unknown as MRTEntryDataEvent[];
      default:
        return [];
    }
  }

  public getMRTEntries(
    _: AbortController,
    provider: string,
    queryType: RouteQueryType,
    target: string,
  ): Promise<ReadableStream> {
    return this.filterData(
      this.dataForProvider(provider),
      queryType,
      target,
    ).then((data) => {
      const ndjson = data
        .map((e) => JSON.stringify({ data: e } as ResumableResponseStreamEvent))
        .join("\n");
      const encoder = new TextEncoder();
      const readable = new ReadableStream({
        start(controller) {
          controller.enqueue(encoder.encode(ndjson));
          controller.close();
        },
      });
      return readable;
    });
  }

  public getMRTEntriesByCursor(
    _ac: AbortController,
    _provider: string,
    _cursorId: string,
  ): Promise<ReadableStream> {
    // Mocked lister does not support cursor-based pagination.
    return Promise.resolve(
      new ReadableStream({
        start(controller) {
          controller.close();
        },
      }),
    );
  }
}

export class DBMRTEntriesLister implements MRTEntriesLister {
  public constructor(public readonly apiPrefix: string) {}

  private static readonly DEFAULT_PAGE_SIZE = 50;

  private buildURL(provider: string, searchParams: URLSearchParams): string {
    searchParams.set("page_size", String(DBMRTEntriesLister.DEFAULT_PAGE_SIZE));
    return `${this.apiPrefix}/mrt_entries/query/${provider}?${searchParams}`;
  }

  // Returns an ndjson stream, where each line is a JSON-encoded ResumableResponseStreamEvent.
  public getMRTEntries(
    abortController: AbortController,
    provider: string,
    queryType: RouteQueryType,
    target: string,
  ): Promise<ReadableStream> {
    const searchParams = new URLSearchParams();

    switch (queryType) {
      case RouteQueryType.Origin_ASN: {
        const asn = parseInt(target, 10);
        if (isNaN(asn) || asn < 0 || !Number.isInteger(asn)) {
          return Promise.reject(new Error(`invalid origin ASN: ${target}`));
        }
        searchParams.set("originAsn", target);
        break;
      }
      case RouteQueryType.Neighbor_ASN: {
        const asn = parseInt(target, 10);
        if (isNaN(asn) || asn < 0 || !Number.isInteger(asn)) {
          return Promise.reject(new Error(`invalid neighbor ASN: ${target}`));
        }
        searchParams.set("neighborAsn", target);
        break;
      }
      case RouteQueryType.AS_Path_Segs: {
        const parts = target.split(",").map((s) => s.trim());
        for (const part of parts) {
          const asn = parseInt(part, 10);
          if (part === "" || isNaN(asn) || asn < 0 || !Number.isInteger(asn)) {
            return Promise.reject(
              new Error(`invalid AS path segment: ${target}`),
            );
          }
        }
        searchParams.set("asSegments", parts.join(","));
        break;
      }
      case RouteQueryType.IP_Or_CIDR: {
        if (isValidCIDR(target)) {
          searchParams.set("cidr", target);
        } else if (isValid(target)) {
          searchParams.set("ip", target);
        } else {
          return Promise.reject(new Error(`invalid IP or CIDR: ${target}`));
        }
        break;
      }
      default:
        return Promise.reject(new Error(`unknown query type: ${queryType}`));
    }

    const url = this.buildURL(provider, searchParams);
    return fetch(url, { signal: abortController.signal }).then(
      (res) => res.body!,
    );
  }

  public getMRTEntriesByCursor(
    abortController: AbortController,
    provider: string,
    cursorId: string,
  ): Promise<ReadableStream> {
    const url =
      `${this.apiPrefix}/mrt_entries/query/${provider}` +
      `?cursor_id=${encodeURIComponent(cursorId)}` +
      `&page_size=${DBMRTEntriesLister.DEFAULT_PAGE_SIZE}`;
    return fetch(url, { signal: abortController.signal }).then(
      (res) => res.body!,
    );
  }
}

export interface ProviderMRTEntriesState {
  entries: MRTEntry[];
  isRunning: boolean;
  error: string | undefined;
  cursorId: string | undefined;
  pageEvents: ResumableResponseStreamEvent[];
}

export type ProviderEntriesMap = Record<string, ProviderMRTEntriesState>;

// --- Stream consumer shared between initial fetch and loadMore ---

function processMRTStream(
  streamPromise: Promise<ReadableStream>,
  signal: AbortSignal,
  provider: string,
  setProviderMap: React.Dispatch<React.SetStateAction<ProviderEntriesMap>>,
): void {
  const alive = () => !signal.aborted;

  streamPromise
    .then((stream) => {
      if (!alive()) return null;
      return stream
        .pipeThrough(new TextDecoderStream())
        .pipeThrough(new LineTokenizer())
        .pipeThrough(new JSONLineDecoder())
        .getReader();
    })
    .then(async (reader) => {
      if (!reader) {
        if (alive()) {
          setProviderMap((prev) => {
            const cur = prev[provider];
            if (!cur) return prev;
            return { ...prev, [provider]: { ...cur, isRunning: false } };
          });
        }
        return;
      }
      while (true) {
        try {
          const { value, done } = await reader.read();
          if (!alive()) {
            reader.cancel().catch(() => {});
            return;
          }
          if (done) {
            setProviderMap((prev) => {
              const cur = prev[provider];
              if (!cur) return prev;
              return { ...prev, [provider]: { ...cur, isRunning: false } };
            });
            return;
          }
          const event = value as ResumableResponseStreamEvent;
          const inner = event.data;
          setProviderMap((prev) => {
            const cur = prev[provider];
            if (!cur) return prev;
            return {
              ...prev,
              [provider]: {
                ...cur,
                entries: inner.Data
                  ? [...cur.entries, inner.Data]
                  : cur.entries,
                error: inner.Err ?? cur.error,
                cursorId: event.cursor_id ?? cur.cursorId,
                pageEvents: [...cur.pageEvents, event],
              },
            };
          });
        } catch (err) {
          if (!alive()) return;
          console.error("failed to read:", err);
          setProviderMap((prev) => {
            const cur = prev[provider];
            if (!cur) return prev;
            return { ...prev, [provider]: { ...cur, isRunning: false } };
          });
          return;
        }
      }
    })
    .catch((err) => {
      if (err.name === "AbortError") {
        console.debug("Stream stopped by user or component unmount.");
      } else if (alive()) {
        console.error("Stream error:", err);
        setProviderMap((prev) => {
          if (!prev[provider]) return prev;
          return {
            ...prev,
            [provider]: {
              ...prev[provider],
              error: err.message,
              isRunning: false,
            },
          };
        });
      }
    });
}

// --- Hook ---

export function useMRTEntriesReadByProvider(
  lister: MRTEntriesLister,
  providers: string[],
  queryType: RouteQueryType | undefined,
  target: string | undefined,
  generation: number,
): {
  providerMap: ProviderEntriesMap;
  loadMore: (provider: string) => void;
} {
  const [providerMap, setProviderMap] = useState<ProviderEntriesMap>({});
  const providerMapRef = useRef(providerMap);
  providerMapRef.current = providerMap;
  const loadMoreAcRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!target || target.trim() === "") {
      setProviderMap({});
      return;
    }

    if (providers.length === 0) {
      return;
    }

    // Cancel any pending loadMore when the main query restarts.
    loadMoreAcRef.current?.abort();
    loadMoreAcRef.current = null;

    // Initialize state for each provider
    const initial: ProviderEntriesMap = {};
    for (const p of providers) {
      initial[p] = {
        entries: [],
        isRunning: true,
        error: undefined,
        cursorId: undefined,
        pageEvents: [],
      };
    }
    setProviderMap(initial);

    const abortController = new AbortController();
    const { signal } = abortController;
    const qt = queryType ?? defaultRouteQueryType;

    for (const provider of providers) {
      processMRTStream(
        lister.getMRTEntries(abortController, provider, qt, target),
        signal,
        provider,
        setProviderMap,
      );
    }

    return () => {
      abortController.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lister, providers.join(","), queryType, target, generation]);

  const loadMore = useCallback(
    (provider: string) => {
      const state = providerMapRef.current[provider];
      if (!state?.cursorId || state.isRunning) return;

      loadMoreAcRef.current?.abort();
      const ac = new AbortController();
      loadMoreAcRef.current = ac;

      const cursorId = state.cursorId;

      // Clear current page state for this provider (entries are already
      // saved by the caller into loadedPagesData before invoking this).
      setProviderMap((prev) => ({
        ...prev,
        [provider]: {
          entries: [],
          isRunning: true,
          error: undefined,
          cursorId: undefined,
          pageEvents: [],
        },
      }));

      processMRTStream(
        lister.getMRTEntriesByCursor(ac, provider, cursorId),
        ac.signal,
        provider,
        setProviderMap,
      );
    },
    [lister],
  );

  return { providerMap, loadMore };
}
