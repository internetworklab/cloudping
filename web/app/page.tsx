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
  Tooltip,
  IconButton,
  Link,
  FormControl,
  FormLabel,
  Paper,
} from "@mui/material";
import { CSSProperties, Fragment, useState } from "react";
import { DNSProbePlan, expandDNSProbePlan, PendingTask } from "@/apis/types";
import { generateRandomTaskId } from "@/apis/random";
import { TaskConfirmDialog } from "@/components/taskconfirm";
import { PingResultDisplay } from "@/components/pingdisplay";
import { TracerouteResultDisplay } from "@/components/traceroutedisplay";
import GitHubIcon from "@mui/icons-material/GitHub";
import MoreHorizIcon from "@mui/icons-material/MoreHoriz";
import { About } from "@/components/about";
import { DNSProbeDisplay } from "@/components/dnsprobedisplay";
import TelegramIcon from "@mui/icons-material/Telegram";
import { testIP } from "@/components/testip";
import { SiteName } from "@/components/sitename";
import { HTTPProbeDisplay } from "@/components/httpprobedisplay";
import { ModeSelector } from "@/components/ModeSelector";
import {
  DNSProbeTaskPanel,
  DNSProbeTransportSelect,
} from "@/components/DNSProbeOptions";
import { PingTaskSourceSelector } from "@/components/PingTaskSourceSelector";
import { PingTaskDefaultTransportOptionsPanel } from "@/components/PingTaskTransportOptions";

function dedup(arr: string[]): string[] {
  return Array.from(new Set(arr));
}

export default function Home() {
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

  const [targetsInput, setTargetsInput] = useState<string>("");
  const targetAttributes = testIP(targetsInput);
  const isNeo = targetAttributes.isNeoIP || targetAttributes.isNeoDomain;
  const isDN42 = targetAttributes.isDN42IP || targetAttributes.isDN42Domain;
  const targetLabelOverrides = isDN42
    ? "DN42 Target"
    : isNeo
      ? "NeoNetwork Target"
      : "Target";

  const [onGoingTasks, setOnGoingTasks] = useState<PendingTask[]>([
    {
      sources: [],
      targets: [],
      taskId: "11451",
      type: "http",
      dnsProbePlan: {
        transport: "udp",
        type: "a",
        domains: [],
        resolvers: [],
      },
    },
  ]);

  let containerStyles: CSSProperties[] = [
    {
      position: "relative",
      left: 0,
      top: 0,
      height: "100vh",
      width: "100vw",
      overflow: "auto",
    },
  ];

  const headerCardStyles: CSSProperties[] = [
    {
      padding: 2,
      display: "flex",
      flexDirection: "column",
      gap: 2,
      position: "relative",
      marginTop: 8,
      minWidth: "68vw",
    },
  ];

  if (onGoingTasks.length === 0) {
    containerStyles = [
      ...containerStyles,
      { display: "flex", justifyContent: "center", alignItems: "center" },
    ];
  }

  const repoAddr = process.env["NEXT_PUBLIC_GITHUB_REPO"];
  const tgInviteLink = process.env["NEXT_PUBLIC_TG_INVITE_LINK"];
  const [showAboutDialog, setShowAboutDialog] = useState<boolean>(false);

  return (
    <Box>
      <Paper
        sx={{
          position: "fixed",
          top: 0,
          left: 0,
          width: "100%",
          paddingLeft: 2,
          paddingRight: 2,
          paddingTop: 1,
          paddingBottom: 1,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 1,
          zIndex: 1,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
          {repoAddr !== "" && (
            <Tooltip title="Go to Project's Github Page">
              <Link
                underline="hover"
                href={repoAddr}
                target="_blank"
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
              >
                <GitHubIcon />
                Project
              </Link>
            </Tooltip>
          )}
          {tgInviteLink!! && (
            <Tooltip title={"Join our Telegram group"}>
              <Link
                underline="hover"
                href={tgInviteLink}
                target={"_blank"}
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
              >
                <TelegramIcon />
                Chat
              </Link>
            </Tooltip>
          )}
          <Tooltip title="More">
            <IconButton size="small" onClick={() => setShowAboutDialog(true)}>
              <MoreHorizIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            flexWrap: "wrap",
            gap: 1,
          }}
        >
          <ModeSelector />
        </Box>
      </Paper>
      <Box sx={containerStyles}>
        <Box sx={headerCardStyles}>
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

                      const dnsTgts =
                        expandDNSProbePlan(newDnsProbePlan).targets;

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
                      pendingTask.type === "ping"
                        ? "Targets"
                        : targetLabelOverrides
                    }
                    value={targetsInput}
                    onChange={(e) => setTargetsInput(e.target.value)}
                  />
                )}
              </Box>
            </CardContent>
          </Card>
          {onGoingTasks.map((task) => (
            <Fragment key={task.taskId}>
              {task.type === "traceroute" ? (
                <TracerouteResultDisplay
                  task={task}
                  onDeleted={() => {
                    setOnGoingTasks(
                      onGoingTasks.filter((t) => t.taskId !== task.taskId),
                    );
                  }}
                />
              ) : task.type === "dns" ? (
                <DNSProbeDisplay
                  task={task}
                  onDeleted={() => {
                    setOnGoingTasks(
                      onGoingTasks.filter((t) => t.taskId !== task.taskId),
                    );
                  }}
                />
              ) : task.type === "http" ? (
                <HTTPProbeDisplay
                  task={task}
                  onDeleted={() => {
                    setOnGoingTasks(
                      onGoingTasks.filter((t) => t.taskId !== task.taskId),
                    );
                  }}
                />
              ) : (
                <PingResultDisplay
                  pendingTask={task}
                  onDeleted={() => {
                    setOnGoingTasks(
                      onGoingTasks.filter((t) => t.taskId !== task.taskId),
                    );
                  }}
                />
              )}
            </Fragment>
          ))}
        </Box>
        <Box sx={{ height: "100vh" }}></Box>
        <TaskConfirmDialog
          pendingTask={pendingTask}
          open={openTaskConfirmDialog}
          onCancel={() => {
            setOpenTaskConfirmDialog(false);
          }}
          onConfirm={() => {
            setOnGoingTasks([pendingTask, ...onGoingTasks]);
            setOpenTaskConfirmDialog(false);
          }}
        />
        <About
          open={showAboutDialog}
          onClose={() => setShowAboutDialog(false)}
        />
      </Box>
    </Box>
  );
}
