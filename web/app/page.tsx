"use client";

import { Box } from "@mui/material";
import { Fragment, useCallback, useState } from "react";
import { PendingTask } from "@/apis/types";

import { PingResultDisplay } from "@/components/pingdisplay";
import { TracerouteResultDisplay } from "@/components/traceroutedisplay";
import { DNSProbeDisplay } from "@/components/dnsprobedisplay";
import { HTTPProbeDisplay } from "@/components/httpprobedisplay";
import { HeaderBar } from "@/components/HeaderBar";
import { TaskCreatorPanel } from "@/components/TaskCreatorPanel";

export default function Home() {
  const [onGoingTasks, setOnGoingTasks] = useState<PendingTask[]>([]);

  const handleTaskDelete = useCallback((taskId: string) => {
    setOnGoingTasks((prev) => prev.filter((t) => t.taskId !== taskId));
  }, []);

  return (
    <Box>
      <HeaderBar />
      <Box
        sx={{
          position: "relative",
          left: 0,
          top: 0,
          height: "100vh",
          width: "100vw",
          overflow: "auto",
          ...(onGoingTasks.length === 0
            ? {
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
              }
            : {}),
        }}
      >
        <Box
          sx={{
            padding: 2,
            display: "flex",
            flexDirection: "column",
            gap: 2,
            position: "relative",
            marginTop: 8,
            minWidth: "68vw",
          }}
        >
          <TaskCreatorPanel
            onNewTaskConfirm={(task) => {
              setOnGoingTasks((onGoingTasks) => [task, ...onGoingTasks]);
            }}
          />

          {onGoingTasks.map((task) => (
            <Fragment key={task.taskId}>
              {task.type === "traceroute" ? (
                <TracerouteResultDisplay
                  task={task}
                  onDeleted={() => handleTaskDelete(task.taskId)}
                />
              ) : task.type === "dns" ? (
                <DNSProbeDisplay
                  task={task}
                  onDeleted={() => handleTaskDelete(task.taskId)}
                />
              ) : task.type === "http" ? (
                <HTTPProbeDisplay
                  task={task}
                  onDeleted={() => handleTaskDelete(task.taskId)}
                />
              ) : (
                <PingResultDisplay
                  pendingTask={task}
                  onDeleted={() => handleTaskDelete(task.taskId)}
                />
              )}
            </Fragment>
          ))}
        </Box>
        <Box sx={{ height: "100vh" }}></Box>
      </Box>
    </Box>
  );
}
