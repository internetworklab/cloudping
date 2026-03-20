"use client";

import { Box, Card, Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { JSONLineDecoder, LineTokenizer } from "@/apis/globalping";
import { EventAdapter } from "@/apis/httpprobe";
import { PendingTask, EventObject } from "@/apis/types";
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
    queryKey: ["/example_http_probe_1.json"],
    queryFn: () =>
      fetch("/example_http_probe_1.json")
        .then((r) => r.body)
        .then((r) => {
          return r
            ?.pipeThrough(new TextDecoderStream())
            .pipeThrough(new LineTokenizer())
            .pipeThrough(new JSONLineDecoder())
            .pipeThrough(new EventAdapter())
            .getReader();
        }),
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
        gap: 1,
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
        <EventsFilterDisplay
          allDsts={allDsts}
          allSrcs={allSrcs}
          evLabelsFilter={evLabelsFilter}
          setEVLabelsFilter={setEVLabelsFilter}
        />
      </Card>

      <Box
        sx={{
          flex: 1,
          overflow: "hidden",
          flexDirection: "column",
          display: "flex",
        }}
      >
        {isLoading ? (
          <Box>Loading</Box>
        ) : reader ? (
          <EventDock evs={evs} />
        ) : (
          <Box>No Data</Box>
        )}
      </Box>
    </Box>
  );
}
