import { PendingTask } from "@/apis/types";
import {
  FormControl,
  FormLabel,
  FormGroup,
  FormControlLabel,
  Checkbox,
} from "@mui/material";
import { Dispatch, SetStateAction } from "react";

export function PingTaskDefaultTransportOptionsPanel(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;

  return (
    <FormControl>
      <FormLabel>Options</FormLabel>
      <FormGroup row>
        <FormControlLabel
          control={
            <Checkbox
              checked={!!pendingTask.preferV4}
              onChange={(_, ckd) => {
                setPendingTask((prev) => ({
                  ...prev,
                  preferV4: !!ckd,
                  preferV6: ckd ? false : prev.preferV6,
                }));
              }}
            />
          }
          label="Prefer V4"
        />
        <FormControlLabel
          control={
            <Checkbox
              checked={!!pendingTask.preferV6}
              onChange={(_, ckd) => {
                setPendingTask((prev) => ({
                  ...prev,
                  preferV4: ckd ? false : prev.preferV4,
                  preferV6: !!ckd,
                }));
              }}
            />
          }
          label="Prefer V6"
        />
        <FormControlLabel
          control={
            <Checkbox
              disabled={pendingTask.type === "tcpping"}
              checked={!!pendingTask.useUDP}
              onChange={(_, ckd) => {
                setPendingTask((prev) => ({
                  ...prev,
                  useUDP: !!ckd,
                }));
              }}
            />
          }
          label="Use UDP"
        />
        <FormControlLabel
          control={
            <Checkbox
              disabled={pendingTask.type !== "traceroute"}
              checked={pendingTask.type === "traceroute" && !!pendingTask.pmtu}
              onChange={(_, ckd) => {
                setPendingTask((prev) => ({
                  ...prev,
                  pmtu: !!ckd,
                }));
              }}
            />
          }
          label="PMTU"
        />
      </FormGroup>
    </FormControl>
  );
}
