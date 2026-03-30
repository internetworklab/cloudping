export interface Route {
  // e.g.: 2001:db8:1234::/48 -> networkAddr=2001:db8:1234::, prefixLength=48
  networkAddr: IPAddrLike;
  prefixLength: number;
  value?: unknown;
}

interface NodeEntryNextHeader {
  segmentLength: number;
  seg2SubTrees: Record<string, NodeEntry | undefined>;
}

interface NodeEntry {
  nextHeader?: NodeEntryNextHeader;
  routes?: Route[];
}

export enum IPAddressFamily {
  IPv4 = "ipv4",
  IPv6 = "ipv6",
}

export interface IPAddrLike {
  toMask(prefixLen: number): IPAddrLike;
  getFamily(): IPAddressFamily;

  // Note: IPv4 addr must return a Uint8Array of byteLength 4,
  // while an IPv6 addr must return a Uint8Array of byteLength 16.
  getBytes(): Uint8Array;
  getMaskedValue(bitOffset: number, nbits: number): bigint;
}

function getMaxBits(ipAddr: IPAddrLike) {
  const family = ipAddr.getFamily();
  if (family === IPAddressFamily.IPv4) {
    return 32;
  } else if (family === IPAddressFamily.IPv6) {
    return 128;
  } else {
    throw new Error("Invalid IP address family");
  }
}

function doLookup(
  tableEntry: NodeEntry,
  ipAddress: IPAddrLike,
  matchedPrefixLen: number,
  maxBits: number,
): { routes?: Route[]; matchedPrefixLen: number } {
  const currentDefault = {
    routes: tableEntry.routes,
    matchedPrefixLen: matchedPrefixLen,
  };
  const nextHeader = tableEntry.nextHeader;
  if (matchedPrefixLen === maxBits || !nextHeader) {
    return currentDefault;
  }

  if (
    nextHeader.segmentLength <= 0 ||
    nextHeader.segmentLength + matchedPrefixLen > maxBits
  ) {
    throw new Error("Invalid segment length");
  }

  const segment = ipAddress.getMaskedValue(
    matchedPrefixLen,
    nextHeader.segmentLength,
  );
  const nextTb = nextHeader.seg2SubTrees[segment.toString()];
  if (!nextTb) {
    return currentDefault;
  }

  const lookupResult = doLookup(
    nextTb,
    ipAddress,
    matchedPrefixLen + nextHeader.segmentLength,
    maxBits,
  );

  return lookupResult.routes && lookupResult.routes.length > 0
    ? lookupResult
    : currentDefault;
}

export function lookup(
  table: NodeEntry,
  ipAddress: IPAddrLike,
): {
  routes?: Route[];
  networkAddress: IPAddrLike;
  prefixLength: number;
} {
  const { matchedPrefixLen, routes } = doLookup(
    table,
    ipAddress,
    0,
    getMaxBits(ipAddress),
  );

  return {
    networkAddress: ipAddress.toMask(matchedPrefixLen),
    prefixLength: matchedPrefixLen,
    routes: routes,
  };
}

function getTrieTreeSegmentLengths(routes: Route[]): number[] {
  if (routes.length === 0) {
    return [];
  }

  let segmentLengths: number[] = routes
    .map((r) => r.prefixLength)
    .sort((a, b) => a - b);

  for (let i = 1; i < segmentLengths.length; i++) {
    segmentLengths[i] = segmentLengths[i] - segmentLengths[i - 1];
  }
  segmentLengths = segmentLengths.filter((seg) => seg > 0);
  // the trie tree wouldn't have a 'zero-length' segment.

  return segmentLengths;
}

// a tableRoot of undefined means the table is empty
function doInsertRoute(
  tableRoot: NodeEntry | undefined,
  route: Route,
  segments: number[],
  levelIndex: number,
  eatenPrefixLen: number,
): NodeEntry {
  tableRoot = {
    ...(tableRoot ?? {}),
  } as NodeEntry;

  const segment =
    levelIndex < segments.length ? segments[levelIndex] : undefined;
  if (segment === undefined || eatenPrefixLen + segment > route.prefixLength) {
    tableRoot.routes = [...(tableRoot.routes ?? []), route];
    return tableRoot;
  }

  const seg = route.networkAddr
    .getMaskedValue(eatenPrefixLen, segment)
    .toString();
  tableRoot.nextHeader = {
    ...(tableRoot.nextHeader ?? { seg2SubTrees: {}, segmentLength: segment }),
    segmentLength: segment,
    seg2SubTrees: {
      ...(tableRoot.nextHeader?.seg2SubTrees ?? {}),
      [seg]: doInsertRoute(
        tableRoot.nextHeader?.seg2SubTrees?.[seg],
        route,
        segments,
        levelIndex + 1,
        eatenPrefixLen + segment,
      ),
    },
  };
  return tableRoot;
}

export function buildTable(routes: Route[]): NodeEntry | undefined {
  if (routes.length === 0) {
    return undefined;
  }
  let table: NodeEntry | undefined = undefined;
  const segments = getTrieTreeSegmentLengths(routes);
  for (const route of routes) {
    table = doInsertRoute(table, route, segments, 0, 0);
  }
  return table;
}
