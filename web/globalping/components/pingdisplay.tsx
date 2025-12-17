"use client";

import {
  Box,
  Typography,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  IconButton,
  Tooltip,
  TableContainer,
} from "@mui/material";
import { Fragment, useEffect, useRef, useState } from "react";
import { PingSample, generatePingSampleStream } from "@/apis/globalping";
import PauseIcon from "@mui/icons-material/Pause";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import { PendingTask } from "@/apis/types";
import { TaskCloseIconButton } from "@/components/taskclose";

export function PingResultDisplay(props: {
  pendingTask: PendingTask;
  onDeleted: () => void;
}) {
  const { pendingTask, onDeleted } = props;
  const { sources, targets } = pendingTask;

  const [latencyMap, setLatencyMap] = useState<
    Record<string, Record<string, number>>
  >({});
  const getLatency = (
    source: string,
    target: string
  ): number | undefined | null => {
    return latencyMap[target]?.[source];
  };

  const getLatencyColor = (latency: number | null | undefined): string => {
    if (latency === null || latency === undefined) {
      return "inherit"; // Default color for missing data
    }
    if (latency <= 40) {
      return "#4caf50"; // Green for [0-40]ms
    } else if (latency <= 150) {
      return "#ff9800"; // Yellow for (40-150]ms
    } else {
      return "#f44336"; // Red for (150, +inf)ms
    }
  };

  const [running, setRunning] = useState<boolean>(true);

  function launchStream(): [
    ReadableStream<PingSample>,
    ReadableStreamDefaultReader<PingSample>
  ] {
    // const resultStream = generateFakePingSampleStream(sources, targets);
    const resultStream = generatePingSampleStream({
      sources: sources,
      targets: targets,
      intervalMs: 300,
      pktTimeoutMs: 3000,
    });
    const reader = resultStream.getReader();
    const readNext = (props: {
      done: boolean;
      value: PingSample | undefined | null;
    }) => {
      if (props.done) {
        return;
      }

      if (props.value !== undefined && props.value !== null) {
        const sample = props.value;
        const sampleFrom = sample.from;
        const sampleTarget = sample.target;
        const sampleLatency = sample.latencyMs;
        if (sampleLatency !== undefined && sampleLatency !== null) {
          setLatencyMap((prev) => ({
            ...prev,
            [sampleTarget]: {
              ...(prev[sampleTarget] || {}),
              [sampleFrom]: sampleLatency,
            },
          }));
        }
      }

      reader.read().then(readNext);
    };

    reader.read().then(readNext);
    return [resultStream, reader];
  }

  const readerRef = useRef<ReadableStreamDefaultReader<PingSample> | null>(
    null
  );

  function cancelStream() {
    if (readerRef.current) {
      const reader = readerRef.current;
      readerRef.current = null;
      reader.cancel();
    }
  }

  useEffect(() => {
    if (running) {
      const [_, reader] = launchStream();
      readerRef.current = reader;
    }

    return () => {
      cancelStream();
    };
  }, [pendingTask.taskId, running]);

  return (
    <Fragment>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="h6">Task #{pendingTask.taskId}</Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <Tooltip title={running ? "Running" : "Stopped"}>
            <IconButton
              onClick={() => {
                if (running) {
                  cancelStream();
                  setRunning(false);
                } else {
                  setRunning(true);
                }
              }}
            >
              {running ? <PauseIcon /> : <PlayArrowIcon />}
            </IconButton>
          </Tooltip>
          <TaskCloseIconButton
            taskId={pendingTask.taskId}
            onConfirmedClosed={() => {
              onDeleted();
            }}
          />
        </Box>
      </Box>
      <TableContainer sx={{ maxWidth: "100%", overflowX: "auto" }}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Target</TableCell>
              {sources.map((source) => (
                <TableCell key={source}>{source}</TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {targets.map((target) => (
              <TableRow key={target}>
                <TableCell>{target}</TableCell>
                {sources.map((source) => {
                  const latency = getLatency(source, target);
                  return (
                    <TableCell
                      key={source}
                      sx={{
                        color: getLatencyColor(latency),
                        fontWeight: 500,
                        minWidth: 100,
                      }}
                    >
                      {latency !== null && latency !== undefined
                        ? `${latency.toFixed(3)} ms`
                        : "—"}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Fragment>
  );
}
