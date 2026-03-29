export interface Route {
  // e.g.: 2001:db8:1234::/48 -> networkAddr=2001:db8:1234::, prefixLength=48
  networkAddr: IPAddrLike;
  prefixLength: number;
}

export interface NodeEntry {
  segmentLength: number;
  seg2SubTrees: Record<string, NodeEntry | undefined>;
  routes?: Route[];
}

export enum IPAddressFamily {
  IPv4 = "ipv4",
  IPv6 = "ipv6",
}

export interface IPAddrLike {
  toMask(prefixLen: number): IPAddrLike;
  getFamily(): IPAddressFamily;
  getMaskedValue(bitOffset: number, nbits: number): bigint;
}

function doLookup4(
  tableEntry: NodeEntry,
  ipAddress: IPAddrLike,
  matchedPrefixLen: number,
): { routes?: Route[]; matchedPrefixLen: number } {
  if (matchedPrefixLen > 32) {
    throw new Error("Invalid prefix length");
  }
  const currentDefault = {
    routes: tableEntry.routes,
    matchedPrefixLen: matchedPrefixLen,
  };
  if (matchedPrefixLen === 32) {
    return currentDefault;
  }
  if (tableEntry.segmentLength <= 0) {
    throw new Error("Invalid segment length");
  }

  const segment = ipAddress.getMaskedValue(
    matchedPrefixLen,
    tableEntry.segmentLength,
  );
  const nextTb = tableEntry.seg2SubTrees[segment.toString()];
  if (!nextTb) {
    return currentDefault;
  }

  const lookupResult = doLookup4(
    nextTb,
    ipAddress,
    matchedPrefixLen + tableEntry.segmentLength,
  );

  return lookupResult.routes && lookupResult.routes.length > 0
    ? lookupResult
    : currentDefault;
}

export function lookup4(
  table: NodeEntry,
  ipAddress: IPAddrLike,
): {
  routes?: Route[];
  networkAddress: IPAddrLike;
  prefixLength: number;
} {
  const { matchedPrefixLen, routes } = doLookup4(table, ipAddress, 0);
  return {
    networkAddress: ipAddress.toMask(matchedPrefixLen),
    prefixLength: matchedPrefixLen,
    routes: routes,
  };
}
