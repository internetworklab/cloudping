"use client";

import {
  Box,
  Card,
  TableCell,
  TableRow,
  TableHead,
  Table,
  TableContainer,
  Typography,
  TableBody,
  CircularProgress,
  Tooltip,
  IconButton,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import { TaskCloseIconButton } from "./taskclose";
import { AnswersMap, DNSResponse, DNSTarget, PendingTask } from "@/apis/types";
import { Fragment, useEffect, useState } from "react";
import { makeRealDNSResponseStream, RawPingEvent } from "@/apis/globalping";

function RenderError(props: { dnsResponse: DNSResponse }) {
  const { dnsResponse } = props;
  if (dnsResponse.error) {
    if (dnsResponse.err_string) {
      return <Box>Err: {dnsResponse.err_string}</Box>;
    }
    if (dnsResponse.no_such_host) {
      return <Box>Err: No such host</Box>;
    }
    if (dnsResponse.io_timeout) {
      return <Box>Err: IO timeout</Box>;
    }
    return <Box>Err: Unknown error</Box>;
  }
  return <Fragment></Fragment>;
}

function updateAnswersMap(
  answersMap: AnswersMap,
  event: RawPingEvent<DNSResponse>
): AnswersMap {
  const dnsResponse = event.data as any as DNSResponse;
  if (!dnsResponse?.corrId) {
    return answersMap;
  }

  const from = event.metadata?.from || "";
  if (!from) {
    return answersMap;
  }

  const newAnswersMap = { ...answersMap };
  if (!newAnswersMap[from]) {
    newAnswersMap[from] = {};
  }

  newAnswersMap[from] = {
    ...newAnswersMap[from],
    [dnsResponse.corrId]: [dnsResponse],
  };

  return newAnswersMap;
}

function makeFakeDNSResponseStream(
  targets: DNSTarget[],
  sources: string[]
): Promise<ReadableStream<RawPingEvent<DNSResponse>>> {
  const timerWrapper: {
    intervalId: ReturnType<typeof setInterval> | null;
  } = {
    intervalId: null,
  };

  const stream = new ReadableStream<RawPingEvent<DNSResponse>>({
    start(controller) {
      timerWrapper.intervalId = setInterval(() => {
        const target = targets[Math.floor(Math.random() * targets.length)];
        const source = sources[Math.floor(Math.random() * sources.length)];
        let ans = Math.random().toString(36).substring(2, 8);
        ans = ans + "." + "from-" + source + "." + target.addrport;
        const response: DNSResponse = {
          corrId: target.corrId,
          server: target.addrport,
          target: target.target,
          query_type: target.queryType,
          answer_strings: [ans],
        };
        const event: RawPingEvent<DNSResponse> = {
          data: response,
          metadata: {
            from: source,
            target: response.corrId,
          },
        };
        controller.enqueue(event);
      }, 500);
    },
    cancel(readon?: any): Promise<void> {
      if (timerWrapper.intervalId) {
        clearInterval(timerWrapper.intervalId);
        timerWrapper.intervalId = null;
      }
      return Promise.resolve();
    },
  });

  return Promise.resolve(stream);
}

function getDNSResponseStream(
  targets: DNSTarget[],
  sources: string[]
): Promise<ReadableStream<RawPingEvent<DNSResponse>>> {
  return makeRealDNSResponseStream(targets, sources);
  return makeFakeDNSResponseStream(targets, sources);
}

export function DNSProbeDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDeleted } = props;
  const { sources } = task;

  const targets = task.dnsProbeTargets || [];

  const [loading, setLoading] = useState(false);
  const [answers, setAnswers] = useState<AnswersMap>();
  const [generation, setGeneration] = useState(0);

  useEffect(() => {
    const streamRef: {
      stream: ReadableStream<RawPingEvent<DNSResponse>> | null;
      reader: ReadableStreamDefaultReader<RawPingEvent<DNSResponse>> | null;
    } = { stream: null, reader: null };

    const timerRef: {
      timer: ReturnType<typeof setTimeout> | null;
    } = { timer: null };

    const timer = window.setTimeout(() => {
      const sources = task.sources || [];
      const targets = task.dnsProbeTargets || [];

      const streamPromise = getDNSResponseStream(targets, sources);

      streamPromise
        .then((stream) => {
          streamRef.stream = stream;
          const reader = stream.getReader();
          streamRef.reader = reader;
          function doRead({
            done,
            value,
          }: {
            done: boolean;
            value: RawPingEvent<DNSResponse> | undefined;
          }) {
            if (done) {
              return;
            }
            if (value) {
              setAnswers((answers) => updateAnswersMap(answers ?? {}, value));
              setLoading(true);
              if (timerRef.timer) {
                const timer = timerRef.timer;
                timerRef.timer = null;
                clearTimeout(timer);
              }
              timerRef.timer = setTimeout(() => setLoading(false), 1000);
            }
            reader.read().then(doRead);
          }
          reader.read().then(doRead);
        })
        .catch((e) => console.error("failed to then stream", e));
    });

    return () => {
      window.clearTimeout(timer);
      if (streamRef.reader) {
        streamRef.reader?.cancel().then(() => {
          streamRef.reader = null;
        });
      }
    };
  }, [task.taskId, setAnswers, generation]);

  return (
    <Card>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          overflow: "hidden",
          padding: 2,
        }}
      >
        <Typography variant="h6">Task #{task.taskId}</Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          {loading && <CircularProgress size={20} />}
          <Tooltip title={"Refresh"}>
            <IconButton
              onClick={() => {
                setGeneration(generation + 1);
              }}
            >
              <RefreshIcon />
            </IconButton>
          </Tooltip>
          <TaskCloseIconButton
            taskId={0}
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
            {targets.map((tgt) => (
              <TableRow key={tgt.corrId}>
                <TableCell>
                  <Box>
                    <Box>Server: {tgt.addrport}</Box>
                    <Box>Query: {tgt.target}</Box>
                    <Box>Type: {tgt.queryType.toUpperCase()}</Box>
                  </Box>
                </TableCell>
                {sources.map((s) => {
                  const responses = answers?.[s]?.[tgt.corrId];
                  if (!responses) {
                    return <TableCell key={s}>{"(No Data)"}</TableCell>;
                  }
                  if (responses.length === 0) {
                    return <TableCell key={s}>{"(No Data)"}</TableCell>;
                  }

                  return (
                    <TableCell key={s}>
                      {responses.map((r, idx) => (
                        <Fragment key={idx}>
                          {r.error ? (
                            <RenderError dnsResponse={r} />
                          ) : (
                            r.answer_strings?.map((ans, ansidx) => (
                              <Box key={ansidx}>{ans}</Box>
                            ))
                          )}
                        </Fragment>
                      ))}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}
