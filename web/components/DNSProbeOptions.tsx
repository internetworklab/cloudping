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
} from "@mui/material";
import { DNSQueryType, PendingTask } from "@/apis/types";
import { Dispatch, SetStateAction } from "react";
import RadioIcon from "@mui/icons-material/Radio";

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
              transport: e.target.value as "udp" | "tcp",
            },
          }))
        }
      >
        <FormControlLabel control={<RadioIcon />} value="udp" label="UDP" />
        <FormControlLabel control={<RadioIcon />} value="tcp" label="TCP" />
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
      <TextField
        sx={{ marginTop: 2 }}
        variant="standard"
        placeholder="Servers where to send queries, separated by comma, e.g. 8.8.8.8, or [2001:4860:4860::8888]:53"
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
