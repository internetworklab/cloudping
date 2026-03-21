"use client";

import {
  Box,
  TextField,
  FormControl,
  RadioGroup,
  FormLabel,
  FormControlLabel,
  Radio,
  Switch,
} from "@mui/material";
import { defaultHTTPProto, PendingTask } from "@/apis/types";
import { Dispatch, SetStateAction } from "react";

export function HTTPProbeTransportSelect(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;
  return (
    <FormControl>
      <FormLabel>Transport</FormLabel>
      <RadioGroup
        row
        value={pendingTask.selectingHttpTransport || defaultHTTPProto}
        onChange={(e) =>
          setPendingTask((prev) => ({
            ...prev,
            selectingHttpTransport: e.target.value as
              | "http/1.1"
              | "http/2"
              | "http/3",
          }))
        }
      >
        <FormControlLabel
          control={<Radio />}
          value="http/1.1"
          label="HTTP/1.1"
        />
        <FormControlLabel control={<Radio />} value="http/2" label="HTTP/2" />
        <FormControlLabel control={<Radio />} value="http/3" label="HTTP/3" />
      </RadioGroup>
    </FormControl>
  );
}

export function HTTPProbeTaskPanel(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;

  return (
    <Box>
      <FormControlLabel
        control={
          <Switch
            checked={pendingTask.addHeaderSW || false}
            onChange={(e) =>
              setPendingTask((prev) => ({
                ...prev,
                addHeaderSW: e.target.checked,
              }))
            }
          />
        }
        label="Add Headers"
      />
      {pendingTask.addHeaderSW && (
        <TextField
          variant="standard"
          placeholder="Additional headers, one per line, e.g. User-Agent: MyAgent"
          fullWidth
          label="Headers"
          multiline
          rows={3}
          value={pendingTask.headersInput || ""}
          onChange={(e) =>
            setPendingTask((prev) => ({
              ...prev,
              headersInput: e.target.value,
            }))
          }
        />
      )}
      <TextField
        sx={{ marginTop: 2 }}
        variant="standard"
        placeholder="URLs to probe, separated by comma, e.g. https://example.com, https://example.org/path"
        fullWidth
        label="URLs"
        value={pendingTask.targetsInput || ""}
        onChange={(e) =>
          setPendingTask((prev) => ({
            ...prev,
            targetsInput: e.target.value,
          }))
        }
      />
    </Box>
  );
}
