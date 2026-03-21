"use client";

import {
  Box,
  TextField,
  Select,
  MenuItem,
  InputLabel,
  FormControl,
  RadioGroup,
  FormLabel,
  FormControlLabel,
  Radio,
} from "@mui/material";
import { DNSQueryType, DNSTransport, PendingTask } from "@/apis/types";
import { Dispatch, SetStateAction } from "react";

export function DNSProbeTransportSelect(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;
  return (
    <FormControl>
      <FormLabel>Transport</FormLabel>
      <RadioGroup
        row
        value={pendingTask.dnsProbePlan.transport}
        onChange={(e) =>
          setPendingTask((prev) => ({
            ...prev,
            dnsProbePlan: {
              ...prev.dnsProbePlan,
              transport: e.target.value as DNSTransport,
            },
          }))
        }
      >
        <FormControlLabel control={<Radio />} value="udp" label="UDP" />
        <FormControlLabel control={<Radio />} value="tcp" label="TCP" />
        <FormControlLabel control={<Radio />} value="tls" label="TLS" />
      </RadioGroup>
    </FormControl>
  );
}

export function DNSProbeTaskPanel(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;
  return (
    <Box>
      <FormControl fullWidth variant="standard">
        <InputLabel>Type</InputLabel>
        <Select
          label="Type"
          value={pendingTask.dnsProbePlan.type}
          onChange={(e) =>
            setPendingTask((prev) => ({
              ...prev,
              dnsProbePlan: {
                ...prev.dnsProbePlan,
                type: e.target.value as DNSQueryType,
              },
            }))
          }
        >
          <MenuItem value={"a"}>A</MenuItem>
          <MenuItem value={"aaaa"}>AAAA</MenuItem>
          <MenuItem value={"cname"}>CNAME</MenuItem>
          <MenuItem value={"mx"}>MX</MenuItem>
          <MenuItem value={"ns"}>NS</MenuItem>
          <MenuItem value={"ptr"}>PTR</MenuItem>
          <MenuItem value={"txt"}>TXT</MenuItem>
        </Select>
      </FormControl>
      <TextField
        sx={{ marginTop: 2 }}
        variant="standard"
        placeholder="Querying Domains, separated by comma"
        fullWidth
        label="Domains"
        value={pendingTask.dnsProbePlan.domainsInput || ""}
        onChange={(e) =>
          setPendingTask((prev) => ({
            ...prev,
            dnsProbePlan: {
              ...prev.dnsProbePlan,
              domainsInput: e.target.value,
            },
          }))
        }
      />
      {pendingTask.dnsProbePlan?.transport === "tls" && (
        <TextField
          sx={{ marginTop: 2 }}
          variant="standard"
          placeholder="e.g. 1.1.1.1=one.one.one.one, 2001:4860:4860::8888=dns.google"
          fullWidth
          label="ServerName Map"
          value={pendingTask.dnsProbePlan.serverNameMapInput || ""}
          onChange={(e) =>
            setPendingTask((prev) => ({
              ...prev,
              dnsProbePlan: {
                ...prev.dnsProbePlan,
                serverNameMapInput: e.target.value,
              },
            }))
          }
        />
      )}
      <TextField
        sx={{ marginTop: 2 }}
        variant="standard"
        placeholder="e.g. 8.8.8.8, or [2001:4860:4860::8888]:53, no hostname here."
        fullWidth
        label="Resolvers"
        value={pendingTask.dnsProbePlan.resolversInput || ""}
        onChange={(e) =>
          setPendingTask((prev) => ({
            ...prev,
            dnsProbePlan: {
              ...prev.dnsProbePlan,
              resolversInput: e.target.value,
            },
          }))
        }
      />
    </Box>
  );
}
