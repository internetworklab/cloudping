"use client";

import { Box, Card, Paper, Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { JSONLineDecoder, LineTokenizer } from "@/apis/globalping";
import { EventAdapter, generateEventStream } from "@/apis/httpprobe";
import { PendingTask, EventObject, HTTPTarget } from "@/apis/types";
import { firstLetterCap } from "./strings";
import { useState } from "react";
import { EventDock, EventsFilterDisplay, useEVsRead } from "./EventsBrowser";

export function HTTPProbeDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDelete } = props;
  const [evLabelsFilter, setEVLabelsFilter] = useState<Record<string, string>>(
    {},
  );
  const { data: reader, isLoading } = useQuery({
    queryKey: [
      "/example_http_probe_1.json",
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
      const sources: string[] = [];
      const dests: HTTPTarget[] = [];

      // todo: fill in `sources` and `dests` based on `task`

      return generateEventStream(sources, dests);
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
        paddingTop: 1,
      }}
    >
      <Card
        sx={{
          flexShrink: 0,
          padding: 2,
          borderRadius: 0,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}
      >
        <Typography variant="h6">
          {firstLetterCap(task.type)} Task #{task.taskId}
        </Typography>
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
          <EventsFilterDisplay
            allDsts={allDsts}
            allSrcs={allSrcs}
            evLabelsFilter={evLabelsFilter}
            setEVLabelsFilter={setEVLabelsFilter}
          />
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
