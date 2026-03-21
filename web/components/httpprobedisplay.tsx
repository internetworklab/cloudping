"use client";

import { Box, Card, Paper, Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { generateEventStream } from "@/apis/httpprobe";
import { TaskCloseIconButton } from "./taskclose";
import { PendingTask, HTTPTarget } from "@/apis/types";
import { firstLetterCap } from "./strings";
import { useState } from "react";
import { EventDock, EventsFilterDisplay, useEVsRead } from "./EventsBrowser";

export function HTTPProbeDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDeleted } = props;
  const [evLabelsFilter, setEVLabelsFilter] = useState<Record<string, string>>(
    {},
  );
  const { data: reader, isLoading } = useQuery({
    queryKey: [
      "taskType",
      task.type,
      "task",
      task.taskId,
      "taskType",
      task.type,
      "sources",
      task.sources,
      "destinations",
      task.httpProbeTargets,
    ],
    queryFn: () => {
      // return generateFakeEventStream();
      const srcs: string[] = task.sources ?? [];
      const dsts: HTTPTarget[] = task.httpProbeTargets ?? [];
      if (srcs.length + dsts.length === 0) {
        return undefined;
      }

      return generateEventStream(srcs, dsts);
    },
  });
  const { evs, allDsts, allSrcs } = useEVsRead(reader, evLabelsFilter);

  return (
    <Box
      sx={{
        display: "flex",
        overflow: "hidden",
        flexDirection: "column",
        borderRadius: 8,
        maxHeight: "90vh",
      }}
    >
      <Card
        sx={{
          padding: 2,
          flexShrink: 0,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}
      >
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <Typography variant="h6">
            {firstLetterCap(task.type)} Task #{task.taskId}
          </Typography>
          <TaskCloseIconButton
            taskId={task.taskId}
            onConfirmedClosed={() => {
              onDeleted();
            }}
          />
        </Box>
        {!isLoading && allDsts.length + allSrcs.length > 0 && (
          <EventsFilterDisplay
            allDsts={allDsts}
            allSrcs={allSrcs}
            evLabelsFilter={evLabelsFilter}
            setEVLabelsFilter={setEVLabelsFilter}
          />
        )}
      </Card>

      {isLoading ? (
        <Paper
          sx={{
            paddingTop: 5,
            paddingBottom: 10,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          Loading
        </Paper>
      ) : reader ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            flex: "1",
            overflow: "hidden",
          }}
        >
          <EventDock evs={evs} />
        </Box>
      ) : (
        <Paper
          sx={{
            paddingTop: 5,
            paddingBottom: 10,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          No Data
        </Paper>
      )}
    </Box>
  );
}
