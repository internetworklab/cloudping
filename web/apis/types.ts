import { ISO8601Timestamp } from "./common";

export type PingTaskType = "ping" | "traceroute" | "tcpping" | "dns" | "http";

export type DNSTransport = "udp" | "tcp" | "tls" | "http/2" | "http/3";

export type DNSQueryType = "a" | "aaaa" | "cname" | "mx" | "ns" | "ptr" | "txt";

export type DNSTarget = {
  corrId: string;
  addrport: string;
  target: string;
  timeoutMs?: number;
  transport?: DNSTransport;
  queryType: DNSQueryType;
  dotServerName?: string;
};

export type DNSResponse = {
  corrId?: string;
  server?: string;
  target?: string;
  query_type?: DNSQueryType;
  answers?: string[];
  answer_strings?: string[];
  error?: string;
  err_string?: string;
  io_timeout?: boolean;
  no_such_host?: boolean;
  // in the unit of nanoseconds
  elapsed?: number;
  started_at?: ISO8601Timestamp;
  // in the unit of nanoseconds
  timeout_specified?: number;
  transport_used?: DNSTransport;
};

// a map of 'from' -> 'corrId' -> 'DNSResponse'
export type AnswersMap = Record<string, Record<string, DNSResponse[]>>;

export type DNSProbePlan = {
  transport: DNSTransport;
  type: DNSQueryType;
  domains: string[];
  resolvers: string[];
  domainsInput?: string;
  resolversInput?: string;
  serverNameMapInput?: string;
};

function stripResolver(resolver: string): string {
  let striped = resolver.trim();
  if (striped.length === 0) {
    return striped;
  }

  striped = striped.replace(/^tls:\/\//i, "");
  striped = striped.replace(/:\d+$/, "");
  striped = striped.replace(/\]$/, "");
  striped = striped.replace(/^\[/, "");
  return striped;
}

function getServerName(
  resolver: string,
  nameMap: Record<string, string>,
): string | undefined {
  const addr = stripResolver(resolver);
  let serverName = nameMap[addr];
  if (typeof serverName === "string" && serverName.length > 0) {
    serverName = serverName.trim();
    if (serverName.length > 0) {
      return serverName;
    }
  }
  return undefined;
}

export function expandDNSProbePlan(
  plan: DNSProbePlan,
  nameMap: Record<string, string>,
): {
  targets: DNSTarget[];
  targetsMap: Record<string, DNSTarget>;
} {
  const targets: DNSTarget[] = [];
  const targetsMap: Record<string, DNSTarget> = {};
  for (const domain of plan.domains) {
    for (const resolver of plan.resolvers) {
      const target: DNSTarget = {
        corrId: targets.length.toString(),
        addrport: resolver,
        target: domain,
        transport: plan.transport,
        queryType: plan.type,
        dotServerName:
          plan.transport === "tls" ||
          plan.transport === "http/2" ||
          plan.transport === "http/3"
            ? getServerName(resolver, nameMap)
            : undefined,
      };
      targets.push(target);
      targetsMap[target.corrId] = target;
    }
  }

  targets.sort((a, b) => {
    const x = parseInt(a.corrId || "0");
    const y = parseInt(b.corrId || "0");
    return x - y;
  });

  return { targets, targetsMap };
}

export type HTTPProto = "http/1.1" | "http/2" | "http/3";
export const defaultHTTPProto: HTTPProto = "http/1.1";
export type IPPref = "ip4" | "ip6" | "ip";
export const defaultIPPref: IPPref = "ip6";

export interface HTTPTarget {
  url: string;
  correlationId: string;
  extraHeaders?: Record<string, string>;
  proto?: HTTPProto;
  resolver?: string;
  inetFamilyPreference?: IPPref;
}

export type PendingTask = {
  sources: string[];
  targets: string[];
  taskId: string;
  type: PingTaskType;
  preferV4?: boolean;
  preferV6?: boolean;
  useUDP?: boolean;
  pmtu?: boolean;
  dnsProbePlan: DNSProbePlan;
  dnsProbeTargets?: DNSTarget[];
  httpProbeTargets?: HTTPTarget[];
  targetsInput?: string;
  headersInput?: string;
  selectingHttpTransport?: HTTPProto;
  selectingIPPref?: IPPref;
  addHeaderSW?: boolean;
};

export type ExactLocation = {
  Longitude: number;
  Latitude: number;
};

export type BasicIPInfo = {
  ASN?: string;
  Location?: string;
  ISP?: string;

  // country is optional,
  // note, this 'Country' field always means the country name, not the code
  Country?: string;

  // region is optional
  Region?: string;
  // city is optional
  City?: string;
  // exact location is optional
  Exact?: ExactLocation;

  // iso3166 alpha2 country code is optional
  ISO3166Alpha2?: string;
};

export enum HTTPProbeTransportEventType {
  TransportEventTypeConnection = "connection",
  TransportEventTypeDNSLookup = "dns-lookup",
  TransportEventTypeRequest = "request",
  TransportEventTypeRequestHeader = "request-header",
  TransportEventTypeResponse = "response",
  TransportEventTypeResponseHeader = "response-header",
  TransportEventTypeMetadata = "metadata",
}

export enum HTTPProbeTransportEventName {
  TransportEventNameMethod = "method",
  TransportEventNameURL = "url",
  TransportEventNameProto = "proto",
  TransportEventNameDialStarted = "dial-started",
  TransportEventNameDialCompleted = "dial-completed",
  TransportEventNameDNSLookupStarted = "dns-lookup-started",
  TransportEventNameDNSLookupCompleted = "dns-lookup-completed",
  TransportEventNameDNSLookupError = "dns-lookup-error",
  TransportEventNameDialError = "dial-error",
  TransportEventNameRequestLine = "request-line",
  TransportEventNameStatus = "status",
  TransportEventNameTransferEncoding = "transfer-encoding",
  TransportEventNameContentLength = "content-length",
  TransportEventNameContentType = "content-type",
  TransportEventNameRequestHeadersStart = "request-headers-start",
  TransportEventNameRequestHeadersEnd = "request-headers-end",
  TransportEventNameResponseHeadersStart = "response-headers-start",
  TransportEventNameResponseHeadersEnd = "response-headers-end",
  TransportEventNameSkipMalformedResponseHeader = "skip-malformed-response-header",
  TransportEventNameResponseHeaderFieldsTruncated = "response-header-fields-truncated",
  TransportEventNameBodyStart = "body-start",
  TransportEventNameBodyEnd = "body-end",
  TransportEventNameBodyBytesRead = "body-bytes-read",
  TransportEventNameBodyChunkBase64 = "body-chunk-base64",
  TransportEventNameBodyReadTruncated = "body-read-truncated",
}

export type HTTPProbeTransportEvent = {
  Type: HTTPProbeTransportEventType;
  Name: HTTPProbeTransportEventName;
  Value: string;
  Date: ISO8601Timestamp;
};

export type HTTPProbeEvent = {
  transport?: HTTPProbeTransportEvent | null;
  error?: string | null;
  correlationId?: string | null;
};

// Raw event returned by the API
export type RawPingEvent<T = RawPingEventData> = {
  data?: T;
  metadata?: RawPingEventMetadata;
};

export type RawPingEventICMPReply = {
  ICMPTypeV4?: number;
  ICMPTypeV6?: number;
  ID?: number;
  Peer?: string;
  PeerRDNS?: string[];
  PeerASN?: string;
  PeerLocation?: string;
  PeerISP?: string;
  PeerExactLocation?: ExactLocation;

  ReceivedAt?: ISO8601Timestamp;

  // Seq of the reply packet
  Seq?: number;
  // size of icmp, without the ip(v4/v6) header
  Size?: number;
  // TTl of the reply packet
  TTL?: number;

  LastHop?: boolean;

  SetMTUTo?: number;

  PeerIPInfo?: BasicIPInfo;
};

export type RawPingEventData = {
  RTTMilliSecs?: number[];
  RTTNanoSecs?: number[];
  Raw?: RawPingEventICMPReply[];
  ReceivedAt?: ISO8601Timestamp[];
  SentAt?: ISO8601Timestamp;

  // Seq of the sent packet
  Seq?: number;

  // TTL of the sent packet
  TTL?: number;
};

export type RawPingEventMetadata = {
  from?: string;
  target?: string;
};

export const FILTERKEY_FROM = "from";
export const FILTERKEY_CORR_ID = "correlationId";

export interface EventObject {
  id: string;
  timestamp: number;
  message: string;

  // labels are for filtering,
  // e.g. you can select events both satisfy these labels:
  // from=us-lax1,correlationId=http://example.com/
  // Some pre-defined keys as these `FILTERKEY_*` constonts listed above.
  labels?: Record<string, string>;

  // annotations are domain-oriented key-value pairs for displaying
  // bussiness informations in a structural way
  annotations?: Record<string, string>;
}
