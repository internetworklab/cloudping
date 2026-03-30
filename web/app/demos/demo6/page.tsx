"use client";

import { Box, TextField } from "@mui/material";
import { useState } from "react";
import { NetworkDescriptor } from "@/apis/nwdesc";
import { useAddressClassify } from "@/apis/useAddressClassify";

export default function Page() {
  const [input, setInput] = useState("");
  const {
    parsed,
    routes,
    domainDescriptors,
    matchedRouteIds,
    matchedDomainIds,
    isRoutesLoading,
    isDomainLoading,
  } = useAddressClassify(input);

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
        {isRoutesLoading ? (
          <Box ml={1}>Loading...</Box>
        ) : (
          routes.map((route, i) => {
            const desc = route.value as NetworkDescriptor;
            const matched = matchedRouteIds.has(desc.id);
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
          Loaded Domain Descriptors ({domainDescriptors.length}):
        </Box>
        {isDomainLoading ? (
          <Box ml={1}>Loading...</Box>
        ) : (
          domainDescriptors.map((desc, i) => {
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
