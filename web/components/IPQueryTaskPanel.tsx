"use client";

import { PendingTask } from "@/apis/types";
import { Box, TextField } from "@mui/material";
import { Dispatch, SetStateAction } from "react";

export function IPQueryTaskPanel({
  pendingTask,
  setPendingTask,
}: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const targetsInput = pendingTask.targetsInput ?? "";

  return (
    <Box>
      <TextField
        variant="standard"
        fullWidth
        placeholder="IP addresses, separated by comma"
        label="IP Addresses"
        value={targetsInput}
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
