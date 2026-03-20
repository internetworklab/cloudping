"use client";

import { EventsBrowser } from "@/components/EventsBrowser";
import { Box } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { JSONLineDecoder, LineTokenizer } from "@/apis/globalping";
import { EventAdapter } from "@/apis/httpprobe";
import { PendingTask } from "@/apis/types";

export function HTTPProbeDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDelete } = props;
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

  return (
    <Box
      sx={{
        height: "100vh",
        overflow: "hidden",
      }}
    >
      {isLoading ? (
        <Box>Loading</Box>
      ) : reader ? (
        <EventsBrowser reader={reader} />
      ) : (
        <Box>No Data</Box>
      )}
    </Box>
  );
}
