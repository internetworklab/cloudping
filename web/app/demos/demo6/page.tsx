"use client";

import { Box, TextField } from "@mui/material";
import { useState } from "react";
import { IPAddr } from "@/utls/ipaddr";
import { IPAddressFamily, Route, buildTable, lookup } from "@/utls/router";
import { isValidCIDR, parseCIDR } from "ipaddr.js";

interface NetworkDescriptor {
  // uniquely identify each routes, unique across the list (or better, globally unique)
  id: string;

  // could be 192.168.0.0/16, 2001:db8:1234::/48
  prefix: string;

  // name doesn't have to be unique
  name: string;

  description?: string;
}

const mockNWDescriptors: NetworkDescriptor[] = [
  {
    id: "rfc1918-192.168",
    prefix: "192.168.0.0/16",
    name: "RFC1918",
    description: "RFC1918 192.168/16",
  },
  {
    id: "rfc1918-172.16",
    prefix: "172.16.0.0/12",
    name: "RFC1918",
    description: "RFC1918 172.16/12",
  },
  {
    id: "dn42-172.20",
    prefix: "172.20.0.0/14",
    name: "DN42",
    description: "An experimental interconnected BGP network",
  },
  {
    id: "neo-10.127",
    prefix: "10.127.0.0/16",
    name: "NeoNetwork",
    description: "Yet another BGP network interconnected with DN42",
  },
  {
    id: "ipv6-ula-fd00",
    prefix: "fd00::/8",
    name: "IPv6-ULA",
    description: "IPv6 ULA as defined in RFC4193",
  },
  {
    id: "ipv6-ll-fe80",
    prefix: "fe80::/64",
    name: "IPv6-LL",
    description: "IPv6 link-local addresses for link scope communication",
  },
];

function toRoutes(): Route[] {
  const routes: Route[] = [];
  for (const desc of mockNWDescriptors) {
    if (!isValidCIDR(desc.prefix)) {
      continue;
    }
    const [ipObj, prefixLength] = parseCIDR(desc.prefix);
    routes.push({
      networkAddr: new IPAddr(
        new Uint8Array(ipObj.toByteArray()),
        ipObj.kind() === "ipv4" ? IPAddressFamily.IPv4 : IPAddressFamily.IPv6,
      ),
      prefixLength,
      value: desc,
    });
  }
  return routes;
}

const routes = toRoutes();
const table = buildTable(routes);

export default function Page() {
  const [input, setInput] = useState("");
  const parsed = IPAddr.fromString(input);

  const lookupResult = parsed && table ? lookup(table, parsed) : undefined;
  const matchedIds = new Set<string>(
    lookupResult?.routes?.map((r) => (r.value as NetworkDescriptor).id) ?? [],
  );

  return (
    <Box>
      <TextField
        variant="standard"
        label="IP Address"
        value={input}
        onChange={(e) => setInput(e.target.value)}
      />
      {parsed && (
        <Box>
          <Box>Value: {input}</Box>
          <Box>Bytes: [{Array.from(parsed.getBytes()).join(", ")}]</Box>
          <Box>Family: {parsed.getFamily()}</Box>
        </Box>
      )}
      <Box mt={2}>
        <Box fontWeight="bold">Loaded Routes ({routes.length}):</Box>
        {routes.map((route, i) => {
          const desc = route.value as NetworkDescriptor;
          const matched = matchedIds.has(desc.id);
          return (
            <Box
              key={i}
              sx={{
                ml: 1,
                fontWeight: matched ? "bold" : "normal",
                outline: matched ? "2px solid" : "none",
                outlineOffset: "2px",
              }}
            >
              {matched ? "▸ " : ""}
              {desc.prefix} — {desc.name}
              {desc.description && `: ${desc.description}`}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}
