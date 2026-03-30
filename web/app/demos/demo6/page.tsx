"use client";

import { Box, TextField } from "@mui/material";
import { useState } from "react";
import { IPAddr } from "@/utls/ipaddr";
import { IPAddressFamily, Route, buildTable, lookup } from "@/utls/router";
import { isValidCIDR, parseCIDR } from "ipaddr.js";
import {
  NetworkDescriptor,
  DomainDescriptor,
  domainDescriptorLookup,
} from "@/apis/nwdesc";
import { useQuery } from "@tanstack/react-query";

export default function Page() {
  const [input, setInput] = useState("");
  const parsed = IPAddr.fromString(input);

  const { data, isLoading } = useQuery({
    queryKey: ["network-descriptors"],
    queryFn: async () => {
      const res = await fetch("/networkdescriptor.json");
      const descriptors: NetworkDescriptor[] = await res.json();
      const routes: Route[] = [];
      for (const desc of descriptors) {
        if (!isValidCIDR(desc.prefix)) {
          continue;
        }
        const [ipObj, prefixLength] = parseCIDR(desc.prefix);
        routes.push({
          networkAddr: new IPAddr(
            new Uint8Array(ipObj.toByteArray()),
            ipObj.kind() === "ipv4"
              ? IPAddressFamily.IPv4
              : IPAddressFamily.IPv6,
          ),
          prefixLength,
          value: desc,
        });
      }
      return { routes, table: buildTable(routes) };
    },
  });

  const { data: domainData, isLoading: domainIsLoading } = useQuery({
    queryKey: ["domain-descriptors"],
    queryFn: async () => {
      const res = await fetch("/domaindescriptor.json");
      const descriptors: DomainDescriptor[] = await res.json();
      return { descriptors };
    },
  });

  const lookupResult =
    parsed && data?.table ? lookup(data.table, parsed) : undefined;
  const matchedIds = new Set<string>(
    lookupResult?.routes?.map((r) => (r.value as NetworkDescriptor).id) ?? [],
  );

  const matchedDomainIds = new Set<string>(
    !parsed && input && domainData?.descriptors
      ? domainDescriptorLookup(domainData.descriptors, input).map((d) => d.id)
      : [],
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
        <Box fontWeight="bold">Loaded Routes ({data?.routes.length ?? 0}):</Box>
        {isLoading ? (
          <Box ml={1}>Loading...</Box>
        ) : (
          (data?.routes ?? []).map((route, i) => {
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
          })
        )}
      </Box>
      <Box mt={2}>
        <Box fontWeight="bold">
          Loaded Domain Descriptors ({domainData?.descriptors.length ?? 0}):
        </Box>
        {domainIsLoading ? (
          <Box ml={1}>Loading...</Box>
        ) : (
          (domainData?.descriptors ?? []).map((desc, i) => {
            const matched = matchedDomainIds.has(desc.id);
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
                {desc.zone} — {desc.name}
                {desc.description && `: ${desc.description}`}
              </Box>
            );
          })
        )}
      </Box>
    </Box>
  );
}
