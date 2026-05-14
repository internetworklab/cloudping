import { BasicIPInfo } from "./types";
import { LineTokenizer, JSONLineDecoder, getApiEndpoint } from "./globalping";
import { useState, useEffect, useRef } from "react";

// ---------------------------------------------------------------------------
// API prefix
// ---------------------------------------------------------------------------

export function getIPQueryServiceAPIPrefix(): string {
  return (
    process.env["NEXT_PUBLIC_IP_QUERY_API_PREFIX"] ||
    getApiEndpoint() + "/ip-query"
  );
}

// ---------------------------------------------------------------------------
// Wire types (match Go pkg/proxy/ip-query-directory.go)
// ---------------------------------------------------------------------------

/** Matches Go DataResponse[[]string] */
export interface IPQueryProvidersResponse {
  data?: string[];
  error?: string;
}

/** Matches Go QueryResultEntry */
export interface IPQueryResultEntry {
  err?: string | null;
  from: string;
  ip: string;
  result?: BasicIPInfo | null;
}

// ---------------------------------------------------------------------------
// Provider lister interface
// ---------------------------------------------------------------------------

export interface IPQueryProvidersLister {
  getIPQueryProviders(): Promise<IPQueryProvidersResponse>;
}

// ---------------------------------------------------------------------------
// DB-backed provider lister
// ---------------------------------------------------------------------------

export class DBIPQueryProvidersLister implements IPQueryProvidersLister {
  public constructor(public readonly apiPrefix: string) {}

  public getIPQueryProviders(): Promise<IPQueryProvidersResponse> {
    const url = `${this.apiPrefix}/providers`;
    return fetch(url).then((r) => r.json());
  }
}

// ---------------------------------------------------------------------------
// Mocked provider lister
// ---------------------------------------------------------------------------

export class MockedIPQueryProvidersLister implements IPQueryProvidersLister {
  public async getIPQueryProviders(): Promise<IPQueryProvidersResponse> {
    return {
      data: ["ipinfo", "ip2location", "ipregistry", "maxmind", "dn42"],
    };
  }
}

// ---------------------------------------------------------------------------
// Query lister interface
// ---------------------------------------------------------------------------

export interface IPQueryLister {
  /**
   * Query IP information from specified providers (or all if omitted).
   * Returns an ndjson ReadableStream where each line is a JSON-encoded IPQueryResultEntry.
   */
  queryIPs(
    abortController: AbortController,
    ips: string[],
    providers?: string[],
  ): Promise<ReadableStream>;
}

// ---------------------------------------------------------------------------
// DB-backed query lister
// ---------------------------------------------------------------------------

export class DBIPQueryLister implements IPQueryLister {
  public constructor(public readonly apiPrefix: string) {}

  public queryIPs(
    abortController: AbortController,
    ips: string[],
    providers?: string[],
  ): Promise<ReadableStream> {
    const searchParams = new URLSearchParams();
    for (const ip of ips) {
      searchParams.append("ip", ip);
    }
    if (providers && providers.length > 0) {
      for (const provider of providers) {
        searchParams.append("provider", provider);
      }
    }
    const url = `${this.apiPrefix}/query?${searchParams}`;
    return fetch(url, { signal: abortController.signal }).then(
      (res) => res.body!,
    );
  }
}

// ---------------------------------------------------------------------------
// Mocked query lister
// ---------------------------------------------------------------------------

export class MockedIPQueryLister implements IPQueryLister {
  public async queryIPs(
    _abortController: AbortController,
    ips: string[],
    providers?: string[],
  ): Promise<ReadableStream> {
    const allProviders = providers ?? [
      "ipinfo",
      "ip2location",
      "ipregistry",
      "maxmind",
      "dn42",
    ];

    const entries: IPQueryResultEntry[] = [];
    for (const ip of ips) {
      for (const provider of allProviders) {
        entries.push({
          from: provider,
          ip,
          result: {
            ASN: "AS65001",
            ISP: `Mocked ISP (${provider})`,
            Location: "Mocked City, MC",
            Country: "Mocked Country",
            Region: "Mocked Region",
            City: "Mocked City",
          },
        });
      }
    }

    const ndjson = entries.map((e) => JSON.stringify(e)).join("\n") + "\n";
    const encoder = new TextEncoder();
    const readable = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode(ndjson));
        controller.close();
      },
    });
    return readable;
  }
}

// ---------------------------------------------------------------------------
// Per-provider state
// ---------------------------------------------------------------------------

export interface ProviderIPQueryState {
  results: IPQueryResultEntry[];
  isRunning: boolean;
  error: string | undefined;
}

export type ProviderIPQueryMap = Record<string, ProviderIPQueryState>;

// ---------------------------------------------------------------------------
// Stream consumer
// ---------------------------------------------------------------------------

function processIPQueryStream(
  streamPromise: Promise<ReadableStream>,
  signal: AbortSignal,
  setProviderMap: React.Dispatch<React.SetStateAction<ProviderIPQueryMap>>,
): void {
  const alive = () => !signal.aborted;

  const runningProviders = new Set<string>();

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
            const next = { ...prev };
            for (const provider of runningProviders) {
              const cur = next[provider];
              if (cur) {
                next[provider] = { ...cur, isRunning: false };
              }
            }
            return next;
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
              const next = { ...prev };
              for (const provider of runningProviders) {
                const cur = next[provider];
                if (cur) {
                  next[provider] = { ...cur, isRunning: false };
                }
              }
              return next;
            });
            return;
          }
          const entry = value as IPQueryResultEntry;
          const provider = entry.from;
          runningProviders.add(provider);
          setProviderMap((prev) => {
            const cur = prev[provider] ?? {
              results: [],
              isRunning: true,
              error: undefined,
            };
            return {
              ...prev,
              [provider]: {
                ...cur,
                results: [...cur.results, entry],
                error: entry.err ?? cur.error,
                isRunning: true,
              },
            };
          });
        } catch (err) {
          if (!alive()) return;
          console.error("failed to read IP query stream:", err);
          setProviderMap((prev) => {
            const next = { ...prev };
            for (const provider of runningProviders) {
              const cur = next[provider];
              if (cur) {
                next[provider] = { ...cur, isRunning: false };
              }
            }
            return next;
          });
          return;
        }
      }
    })
    .catch((err) => {
      if (err.name === "AbortError") {
        console.debug("IP query stream stopped by user or component unmount.");
      } else if (alive()) {
        console.error("IP query stream error:", err);
        setProviderMap((prev) => {
          const next = { ...prev };
          for (const provider of runningProviders) {
            const cur = next[provider];
            if (cur) {
              next[provider] = {
                ...cur,
                error: err.message,
                isRunning: false,
              };
            }
          }
          return next;
        });
      }
    });
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useIPQueryByProvider(
  lister: IPQueryLister,
  ips: string[],
  providers: string[] | undefined,
  generation: number,
): ProviderIPQueryMap {
  const [providerMap, setProviderMap] = useState<ProviderIPQueryMap>({});
  const providerMapRef = useRef(providerMap);
  providerMapRef.current = providerMap;

  useEffect(() => {
    if (ips.length === 0) {
      setProviderMap({});
      return;
    }

    // Initialize state for each known provider
    const initial: ProviderIPQueryMap = {};
    if (providers && providers.length > 0) {
      for (const p of providers) {
        initial[p] = {
          results: [],
          isRunning: true,
          error: undefined,
        };
      }
    }
    setProviderMap(initial);

    const abortController = new AbortController();

    processIPQueryStream(
      lister.queryIPs(abortController, ips, providers),
      abortController.signal,
      setProviderMap,
    );

    return () => {
      abortController.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lister, ips.join(","), providers?.join(","), generation]);

  return providerMap;
}
