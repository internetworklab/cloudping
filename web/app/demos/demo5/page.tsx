"use client";

import { Box, TextField } from "@mui/material";
import { useState } from "react";
import { testIP } from "@/components/testip";

export default function Page() {
  const [v, setV] = useState("");

  const {
    isDN42IPv4,
    isDN42IPv6,
    isDN42IP,
    isNeoV4,
    isNeoV6,
    isNeoIP,
    isValidIP,
    isNeoDomain,
    isDN42Domain,
  } = testIP(v);

  return (
    <Box>
      <TextField
        variant="standard"
        value={v}
        onChange={(e) => setV(e.target.value)}
      />
      <Box>IsValidIP: {String(isValidIP)}</Box>
      <Box>IsDN42V4: {String(isDN42IPv4)}</Box>
      <Box>IsDN42V6: {String(isDN42IPv6)}</Box>
      <Box>IsDN42IP: {String(isDN42IP)}</Box>
      <Box>IsNeoV4: {String(isNeoV4)}</Box>
      <Box>IsNeoV6: {String(isNeoV6)}</Box>
      <Box>IsNeo: {String(isNeoIP)}</Box>
      <Box>IsNeoDomain: {String(isNeoDomain)}</Box>
      <Box>IsDN42Domain: {String(isDN42Domain)}</Box>
    </Box>
  );
}
