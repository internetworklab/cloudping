"use client";

import {
  Box,
  Card,
  Typography,
  CardContent,
  TextField,
  Button,
  FormControlLabel,
  RadioGroup,
  Radio,
  FormControl,
  FormLabel,
} from "@mui/material";
import { DNSProbePlan, expandDNSProbePlan, PendingTask } from "@/apis/types";
import { generateRandomTaskId } from "@/apis/random";
import { SiteName } from "@/components/sitename";
import {
  DNSProbeTaskPanel,
  DNSProbeTransportSelect,
} from "@/components/DNSProbeOptions";
import { PingTaskSourceSelector } from "@/components/PingTaskSourceSelector";
import { PingTaskDefaultTransportOptionsPanel } from "@/components/PingTaskTransportOptions";
import { dedup } from "@/apis/utils";
import { Fragment, useState } from "react";
import { testIP } from "@/components/testip";
import { TaskConfirmDialog } from "@/components/taskconfirm";

export function TaskCreatorPanel(props: {
  onNewTaskConfirm: (task: PendingTask) => void;
}) {
  const [pendingTask, setPendingTask] = useState<PendingTask>(() => {
    return {
      sources: [],
      targets: [],
      taskId: "",
      type: "ping",
      dnsProbePlan: {
        transport: "udp",
        type: "a",
        domains: [],
        resolvers: [],
      },
    };
  });
  const [openTaskConfirmDialog, setOpenTaskConfirmDialog] =
    useState<boolean>(false);
  const { onNewTaskConfirm } = props;

  const [targetsInput, setTargetsInput] = useState<string>("");
  const targetAttributes = testIP(targetsInput);

  const isNeo = targetAttributes.isNeoIP || targetAttributes.isNeoDomain;
  const isDN42 = targetAttributes.isDN42IP || targetAttributes.isDN42Domain;

  const targetLabelOverrides = isDN42
    ? "DN42 Target"
    : isNeo
      ? "NeoNetwork Target"
      : "Target";

  return (
    <Fragment>
      <Card>
        <CardContent>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              flexWrap: "wrap",
              gap: 2,
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 2,
                flexWrap: "wrap",
              }}
            >
              <Typography variant="h6">
                <SiteName />
              </Typography>
            </Box>

            <Button
              variant="contained"
              color="primary"
              onClick={() => {
                const tgts = dedup(targetsInput.split(","))
                  .map((t) => t.trim())
                  .filter((t) => t.length > 0);

                const domains = dedup(
                  pendingTask.dnsProbePlan.domainsInput?.split(",") || [],
                )
                  .map((d) => d.trim())
                  .filter((d) => d.length > 0);

                const resolvers = dedup(
                  pendingTask.dnsProbePlan.resolversInput?.split(",") || [],
                )
                  .map((r) => r.trim())
                  .filter((r) => r.length > 0);

                setPendingTask((prev) => {
                  const newDnsProbePlan: DNSProbePlan = {
                    ...pendingTask.dnsProbePlan,
                    domains: domains,
                    resolvers: resolvers,
                  };

                  const dnsTgts = expandDNSProbePlan(newDnsProbePlan).targets;

                  return {
                    ...prev,
                    targets: tgts,
                    taskId: generateRandomTaskId(),
                    dnsProbePlan: newDnsProbePlan,
                    dnsProbeTargets: dnsTgts,
                  };
                });
                setOpenTaskConfirmDialog(true);
              }}
            >
              Add Task
            </Button>
          </Box>
          <Box
            sx={{
              marginTop: 2,
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              flexWrap: "wrap",
              gap: 2,
            }}
          >
            <Box
              sx={{
                display: "flex",
                gap: 2,
                flexWrap: "wrap",
                alignItems: "center",
              }}
            >
              <FormControl>
                <FormLabel>Task Type</FormLabel>
                <RadioGroup
                  value={pendingTask.type}
                  onChange={(e) =>
                    setPendingTask((prev) => ({
                      ...prev,
                      type: e.target.value as "ping" | "traceroute",
                      pmtu:
                        e.target.value === "ping" ||
                        e.target.value === "tcpping"
                          ? false
                          : prev.pmtu,
                      useUDP:
                        e.target.value === "tcpping" ? false : prev.useUDP,
                    }))
                  }
                  row
                >
                  <FormControlLabel
                    value="ping"
                    control={<Radio />}
                    label="Ping"
                  />
                  <FormControlLabel
                    value="traceroute"
                    control={<Radio />}
                    label="Traceroute"
                  />
                  <FormControlLabel
                    value="tcpping"
                    control={<Radio />}
                    label="TCP Ping"
                  />
                  <FormControlLabel
                    value="dns"
                    control={<Radio />}
                    label="DNS"
                  />
                  <FormControlLabel
                    value="http"
                    control={<Radio />}
                    label="HTTP"
                  />
                </RadioGroup>
              </FormControl>
            </Box>
          </Box>
          <Box sx={{ marginTop: 2 }}>
            {pendingTask.type === "dns" ? (
              <DNSProbeTransportSelect
                pendingTask={pendingTask}
                setPendingTask={setPendingTask}
              />
            ) : (
              <PingTaskDefaultTransportOptionsPanel
                pendingTask={pendingTask}
                setPendingTask={setPendingTask}
              />
            )}
          </Box>
          <Box sx={{ marginTop: 2 }}>
            <PingTaskSourceSelector
              pendingTask={pendingTask}
              setPendingTask={setPendingTask}
            />
          </Box>
          <Box sx={{ marginTop: 2 }}>
            {pendingTask.type === "dns" ? (
              <DNSProbeTaskPanel
                pendingTask={pendingTask}
                setPendingTask={setPendingTask}
              />
            ) : (
              <TextField
                variant="standard"
                placeholder={
                  pendingTask.type === "ping"
                    ? "Targets, separated by comma"
                    : pendingTask.type === "tcpping"
                      ? 'Specify a single target, in the format of <host>:<port>", e.g. 192.168.1.1:80, or cloudflare.com:443'
                      : "Specify a single target"
                }
                fullWidth
                label={
                  pendingTask.type === "ping" ? "Targets" : targetLabelOverrides
                }
                value={targetsInput}
                onChange={(e) => setTargetsInput(e.target.value)}
              />
            )}
          </Box>
        </CardContent>
      </Card>
      <TaskConfirmDialog
        pendingTask={pendingTask}
        open={openTaskConfirmDialog}
        onCancel={() => {
          setOpenTaskConfirmDialog(false);
        }}
        onConfirm={() => {
          onNewTaskConfirm(pendingTask);
          setOpenTaskConfirmDialog(false);
        }}
      />
    </Fragment>
  );
}
