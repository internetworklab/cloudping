"use client";

import {
  Typography,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from "@mui/material";
import { Fragment } from "react";
import { PendingTask } from "@/apis/types";

export function TaskConfirmDialog(props: {
  pendingTask: PendingTask;
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const { open, pendingTask, onConfirm, onCancel } = props;

  if (pendingTask.type === "traceroute" && pendingTask.targets.length > 1) {
    return (
      <Dialog maxWidth="sm" fullWidth open={open} onClose={onCancel}>
        <DialogTitle>Note</DialogTitle>
        <DialogContent>
          For traceroute task, only one target at a time.
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancel}>Good</Button>
        </DialogActions>
      </Dialog>
    );
  }

  if (pendingTask.sources.length === 0 || pendingTask.targets.length === 0) {
    return (
      <Dialog maxWidth="sm" fullWidth open={open} onClose={onCancel}>
        <DialogTitle>Note</DialogTitle>
        <DialogContent>
          At least one source and one target are required.
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancel}>Good</Button>
        </DialogActions>
      </Dialog>
    );
  }

  return (
    <Fragment>
      <Dialog maxWidth="sm" fullWidth open={open} onClose={onCancel}>
        <DialogTitle>Confirm Task</DialogTitle>
        <DialogContent>
          <Typography gutterBottom>Type: {pendingTask.type}</Typography>
          <Typography gutterBottom>
            Sources: {pendingTask.sources.join(", ")}
          </Typography>
          <Typography>
            {pendingTask.type === "ping" ? "Targets" : "Target"}:{" "}
            {pendingTask.targets.join(", ")}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancel}>Cancel</Button>
          <Button onClick={onConfirm}>Confirm</Button>
        </DialogActions>
      </Dialog>
    </Fragment>
  );
}
