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
} from "@mui/material";
import { PlayPauseButton } from "./playpause";
import { TaskCloseIconButton } from "./taskclose";
import {
  AnswersMap,
  DNSProbePlan,
  DNSResponse,
  expandDNSProbePlan,
  PendingTask,
} from "@/apis/types";
import { Fragment, useState } from "react";

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

export function DNSProbeDisplay(props: { task: PendingTask }) {
  const { task } = props;
  const { sources } = task;

  const targets = task.dnsProbeTargets || [];

  const [loading, setLoading] = useState(false);
  const [answers, setAnswers] = useState<AnswersMap>();

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
        <Typography variant="h6">Task #0</Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          {loading && <CircularProgress size={20} />}
          <TaskCloseIconButton
            taskId={0}
            onConfirmedClosed={() => {
              console.log("onConfirmedClosed");
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
