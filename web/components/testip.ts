"use client";

import { isValid, parseCIDR, parse, IPv4, IPv6 } from "ipaddr.js";

export const dn42Range4 = parseCIDR("172.20.0.0/14");
export const dn42Range6 = parseCIDR("fd00::/8");
export const neoNet4 = parseCIDR("10.127.0.0/16");
export const neoNet6 = parseCIDR("fd10:127::/32");

export type TestResult = {
  isDN42IPv4: boolean;
  isDN42IPv6: boolean;
  isDN42IP: boolean;
  isNeoV4: boolean;
  isNeoV6: boolean;
  isNeoIP: boolean;
  isValidIP: boolean;
  addrObj?: IPv4 | IPv6;
  isNeoDomain: boolean;
  isDN42Domain: boolean;
};

export function testIP(v: string): TestResult {
  const isValidIP = isValid(v);
  const addrObj = isValidIP ? parse(v) : undefined;

  let isDN42IPv4 = false;
  let isDN42IPv6 = false;
  let isDN42IP = false;
  let isNeoV4 = false;
  let isNeoV6 = false;
  let isNeoIP = false;
  let isNeoDomain = false;
  let isDN42Domain = false;
  if (addrObj) {
    if (addrObj.kind() === "ipv4") {
      isDN42IPv4 = !!addrObj.match(dn42Range4);
      isNeoV4 = !!addrObj.match(neoNet4);
    } else if (addrObj.kind() === "ipv6") {
      isDN42IPv6 = !!addrObj.match(dn42Range6);
      isNeoV6 = !!addrObj.match(neoNet6);
    }
    isDN42IP = isDN42IPv4 || isDN42IPv6;
    isNeoIP = isNeoV4 || isNeoV6;
  } else {
    isNeoDomain = v.endsWith(".neo") || v.endsWith(".neo.");
    isDN42Domain = v.endsWith(".dn42") || v.endsWith(".dn42.");
  }

  return {
    isDN42IPv4,
    isDN42IPv6,
    isDN42IP,
    isNeoV4,
    isNeoV6,
    isNeoIP,
    isValidIP,
    addrObj,
    isNeoDomain,
    isDN42Domain,
  };
}
