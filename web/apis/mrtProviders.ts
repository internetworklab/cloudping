import { defaultRouteQueryType, RouteQueryType } from "./types";
import mrtDataFromRirAfrinic from "../public/example_mrt_entries_rir-afrinic.json";
import mrtDataFromRirApnic from "../public/example_mrt_entries_rir-apnic.json";
import mrtDataFromRirArin from "../public/example_mrt_entries_rir-arin.json";
import mrtDataFromRirLacnic from "../public/example_mrt_entries_rir-lacnic.json";
import mrtDataFromRirRipencc from "../public/example_mrt_entries_rir-ripencc.json";
import { parse as parseIP, parseCIDR, isValidCIDR, isValid } from "ipaddr.js";
import { useState, useEffect } from "react";
import { LineTokenizer, JSONLineDecoder } from "./globalping";

export function getMRTEntryServiceAPIPrefix(): string {
  return process.env["NEXT_PUBLIC_ROUTE_INFO_API_PREFIX"] || "/api/proxy/route";
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

export interface MRTEntriesLister {
  // Returns an ndjson stream, where each line is a JSON-encoded MRTEntryDataEvent.
  // think of abortController here like the ctx in golang.
  getMRTEntries(
    abortController: AbortController,
    provider: string,
    queryType: RouteQueryType,
    target: string,
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
      const ndjson = data.map((e) => JSON.stringify(e)).join("\n");
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
}

export class DBMRTEntriesLister implements MRTEntriesLister {
  public constructor(public readonly apiPrefix: string) {}

  private buildURL(provider: string, searchParams: URLSearchParams): string {
    return `${this.apiPrefix}/mrt_entries/query/${provider}?${searchParams}`;
  }

  // Returns an ndjson stream, where each line is a JSON-encoded MRTEntryDataEvent.
  // think of abortController here like the ctx in golang.
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
}

export interface ProviderMRTEntriesState {
  entries: MRTEntry[];
  isRunning: boolean;
  error: string | undefined;
}

export type ProviderEntriesMap = Record<string, ProviderMRTEntriesState>;

export function useMRTEntriesReadByProvider(
  lister: MRTEntriesLister,
  providers: string[],
  queryType: RouteQueryType | undefined,
  target: string | undefined,
  generation: number,
): ProviderEntriesMap {
  const [providerMap, setProviderMap] = useState<ProviderEntriesMap>({});

  useEffect(() => {
    if (!target || target.trim() === "") {
      setProviderMap({});
      return;
    }

    if (providers.length === 0) {
      return;
    }

    // Initialize state for each provider
    const initial: ProviderEntriesMap = {};
    for (const p of providers) {
      initial[p] = { entries: [], isRunning: true, error: undefined };
    }
    setProviderMap(initial);

    const abortController = new AbortController();
    const { signal } = abortController;
    // alive() returns true only while this effect tick is still current.
    const alive = () => !signal.aborted;

    const qt = queryType ?? defaultRouteQueryType;

    for (const provider of providers) {
      lister
        .getMRTEntries(abortController, provider, qt, target)
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
              setProviderMap((prev) => ({
                ...prev,
                [provider]: { ...prev[provider], isRunning: false },
              }));
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
                setProviderMap((prev) => ({
                  ...prev,
                  [provider]: { ...prev[provider], isRunning: false },
                }));
                return;
              }
              const event = value as MRTEntryDataEvent;
              if (event.Err) {
                setProviderMap((prev) => ({
                  ...prev,
                  [provider]: { ...prev[provider], error: event.Err },
                }));
              } else if (event.Data) {
                setProviderMap((prev) => ({
                  ...prev,
                  [provider]: {
                    ...prev[provider],
                    entries: [...prev[provider].entries, event.Data!],
                  },
                }));
              }
            } catch (err) {
              if (!alive()) return;
              console.error("failed to read:", err);
              setProviderMap((prev) => ({
                ...prev,
                [provider]: { ...prev[provider], isRunning: false },
              }));
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

    return () => {
      abortController.abort();
      setProviderMap({});
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lister, providers.join(","), queryType, target, generation]);

  return providerMap;
}
