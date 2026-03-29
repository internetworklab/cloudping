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
  if (matchedPrefixLen === maxBits) {
    return currentDefault;
  }
  if (
    tableEntry.segmentLength <= 0 ||
    tableEntry.segmentLength + matchedPrefixLen > maxBits
  ) {
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

  const lookupResult = doLookup(
    nextTb,
    ipAddress,
    matchedPrefixLen + tableEntry.segmentLength,
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
