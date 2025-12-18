"use client";

import {
  Box,
  Typography,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  TableContainer,
  Tab,
  Tabs,
} from "@mui/material";
import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { TaskCloseIconButton } from "@/components/taskclose";
import { PlayPauseButton } from "./playpause";
import { getLatencyColor } from "./colorfunc";
import { IPDisp } from "./ipdisp";
import { generatePingSampleStream, PingSample } from "@/apis/globalping";
import { PendingTask } from "@/apis/types";
import { demoPingSamples } from "@/apis/mock/mocktraceroute";

type TracerouteIPEntry = {
  ip: string;
  rdns?: string;
};

type TraceroutePeerEntry = {
  seq: number;
  ip: TracerouteIPEntry;
  asn?: string;
  location?: string;
  isp?: string;
};

// unit: milliseconds
type TracerouteRTTStatsEntry = {
  current: number;
  min: number;
  median: number;
  max: number;
  history: number[];
};

type TracerouteStatsEntry = {
  sent: number;
  replied: number;
  lost: number;
};

type HopEntryState = {
  peers: TraceroutePeerEntry[];
  rtts: TracerouteRTTStatsEntry;
  stats: TracerouteStatsEntry;
};

type TabState = {
  maxHop: number;
  hopEntries: Record<number, HopEntryState>;
};
type PageState = Record<string, TabState>;

function sortAndDedupPeers(
  peers: TraceroutePeerEntry[]
): TraceroutePeerEntry[] {
  const sorted = [...peers].sort((a, b) => b.seq - a.seq);
  const m = new Map<string, TraceroutePeerEntry>();
  for (const peer of sorted) {
    if (m.has(peer.ip.ip)) {
      continue;
    }
    m.set(peer.ip.ip, peer);
  }
  return Array.from(m.values()).sort((a, b) => b.seq - a.seq);
}

function getMedian(history: number[]): number {
  if (history.length === 0) {
    return NaN;
  }
  if (history.length % 2 === 0) {
    const lmid_idx = history.length / 2 - 1;
    const rmid_idx = history.length / 2;
    return (history[lmid_idx] + history[rmid_idx]) / 2;
  }
  const mid_idx = Math.floor(history.length / 2);
  return history[mid_idx];
}

function updateHopEntryState(
  hopEntryState: HopEntryState,
  pingSample: PingSample
): HopEntryState {
  const newEntry = { ...hopEntryState };
  if (pingSample.latencyMs !== undefined && pingSample.latencyMs !== null) {
    newEntry.rtts.current = pingSample.latencyMs;
    if (pingSample.latencyMs < newEntry.rtts.min) {
      newEntry.rtts.min = pingSample.latencyMs;
    }
    if (pingSample.latencyMs > newEntry.rtts.max) {
      newEntry.rtts.max = pingSample.latencyMs;
    }
    newEntry.rtts.history = [...newEntry.rtts.history, pingSample.latencyMs];
    newEntry.rtts.median = getMedian(newEntry.rtts.history);
    newEntry.stats.replied++;
  } else {
    newEntry.stats.lost++;
  }
  newEntry.stats.sent++;

  if (pingSample.seq !== undefined && pingSample.seq !== null) {
    const newPeerEntry: TraceroutePeerEntry = {
      ip: {
        ip: pingSample.peer || "",
        rdns: pingSample.peerRdns,
      },
      seq: pingSample.seq,
      asn: pingSample.peerASN,
      location: pingSample.peerLocation,
      isp: pingSample.peerISP,
    };
    newEntry.peers = sortAndDedupPeers([...newEntry.peers, newPeerEntry]);
    // high seq first
    newEntry.peers.sort((a, b) => b.seq - a.seq);
  }

  return newEntry;
}

function updateTabState(tabState: TabState, pingSample: PingSample): TabState {
  const newTabState = { ...tabState };

  if (pingSample.ttl !== undefined && pingSample.ttl !== null) {
    if (pingSample.ttl > newTabState.maxHop) {
      newTabState.maxHop = pingSample.ttl;
    }

    newTabState.maxHop = Math.max(newTabState.maxHop, pingSample.ttl);
    newTabState.hopEntries = {
      ...newTabState.hopEntries,
      [pingSample.ttl]: updateHopEntryState(
        newTabState.hopEntries[pingSample.ttl] ?? {
          peers: [],
          rtts: {
            current: 0,
            min: Infinity,
            median: 0,
            max: -Infinity,
            history: [],
          },
          stats: {
            sent: 0,
            replied: 0,
            lost: 0,
          },
        },
        pingSample
      ),
    };
  }

  return newTabState;
}

function updatePageState(
  pageState: PageState,
  pingSample: PingSample
): PageState {
  // debugger;
  const newState = { ...pageState };

  if (!(pingSample.from in newState)) {
    newState[pingSample.from] = {
      maxHop: 1,
      hopEntries: {},
    };
  }

  newState[pingSample.from] = updateTabState(
    newState[pingSample.from],
    pingSample
  );
  return newState;
}

type DisplayEntry = {
  hop: number;
  entry: HopEntryState;
};

function getDispEntries(
  hopEntries: PageState,
  tabValue: string
): DisplayEntry[] {
  const dispEntries: DisplayEntry[] = [];
  const currentTabEntries = hopEntries[tabValue];
  if (currentTabEntries) {
    for (let i = 1; i <= currentTabEntries.maxHop; i++) {
      if (i in currentTabEntries.hopEntries) {
        dispEntries.push({
          hop: i,
          entry: currentTabEntries.hopEntries[i],
        });
      } else {
        dispEntries.push({
          hop: i,
          entry: {
            peers: [],
            rtts: {
              current: 0,
              min: Infinity,
              median: 0,
              max: -Infinity,
              history: [],
            },
            stats: {
              sent: 0,
              replied: 0,
              lost: 0,
            },
          },
        });
      }
    }
  }
  return dispEntries;
}

export function TracerouteResultDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDeleted } = props;

  const [pageState, setPageState] = useState<PageState>({});
  const [paused, setPaused] = useState<boolean>(false);
  const pausedRef = useRef<boolean>(false);

  const [tabValue, setTabValue] = useState(task.sources[0]);

  const readerRef = useRef<ReadableStreamDefaultReader<PingSample> | null>(
    null
  );

  useEffect(() => {
    console.log("[dbg] useEffect mount");
    if (!readerRef.current) {
      console.log("[dbg] creating stream and getting reader");

      // const stream = streamFromSamples(demoPingSamples);
      const stream = generatePingSampleStream({
        sources: task.sources,
        targets: task.targets.slice(0, 1),
        intervalMs: 300,
        pktTimeoutMs: 3000,
        ttl: "auto",

        // when testing, use 'random', should replace this with 'ipinfo' later
        ipInfoProviderName: "random",
      });

      const reader = stream.getReader();
      readerRef.current = reader;
    }

    return () => {
      console.log("[dbg] useEffect unmount");
      if (readerRef.current) {
        const reader = readerRef.current;
        readerRef.current = null;
        console.log("[dbg] canceling reader");
        reader
          .cancel()
          .then(() => {
            console.log("[dbg] reader cancelled");
          })
          .catch((err) => {
            console.error("[dbg] failed to cancel reader:", err);
          });
      }
    };
  }, [task.taskId]);

  useEffect(() => {
    console.log("[dbg] enter useEffect [paused]", paused);
    const readNext = ({
      done,
      value,
    }: {
      done: boolean;
      value: PingSample | undefined | null;
    }) => {
      if (pausedRef.current) {
        console.log("[dbg] paused, skipping");
        return;
      }
      console.log("[dbg] readNext", done, value);
      if (done) {
        return;
      }
      if (value) {
        setPageState((prev) => updatePageState(prev, value));
        readerRef.current?.read().then(readNext);
      }
    };
    readerRef.current?.read().then(readNext);
    return () => {
      console.log("[dbg] exit useEffect [paused]", paused);
    };
  }, [paused]);

  return (
    <Fragment>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <Typography variant="h6">Task #{1}</Typography>
          {task.sources.length > 0 && (
            <Tabs
              value={tabValue}
              onChange={(event, newValue) => setTabValue(newValue)}
            >
              {task.sources.map((source, idx) => (
                <Tab value={source} label={source} key={idx} />
              ))}
            </Tabs>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <PlayPauseButton
            running={!paused}
            onToggle={(prev, nxt) => {
              if (prev) {
                // prev is running, next is not running
                setPaused(true);
                pausedRef.current = true;
              } else {
                // prev is not running, next is running
                setPaused(false);
                pausedRef.current = false;
              }
            }}
          />

          <TaskCloseIconButton
            taskId={task.taskId}
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
              <TableCell>Hop</TableCell>
              <TableCell>Peers</TableCell>
              <TableCell>RTT</TableCell>
              <TableCell>Stats</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {getDispEntries(pageState, tabValue).map(({ hop, entry }) => {
              return (
                <TableRow key={hop}>
                  <TableCell>{hop}</TableCell>
                  <TableCell>
                    {entry.peers.length > 0 ? (
                      <Box
                        sx={{
                          display: "grid",
                          gridTemplateColumns: "repeat(4, auto)",
                          alignItems: "center",
                          justifyItems: "flex-start",
                          justifyContent: "start",
                          columnGap: 2,
                        }}
                      >
                        {entry.peers.map((peer, idx) => (
                          <Fragment key={idx}>
                            <IPDisp rdns={peer.ip.rdns} ip={peer.ip.ip} />
                            <Box>{peer.asn}</Box>
                            <Box>{peer.location}</Box>
                            <Box>{peer.isp}</Box>
                          </Fragment>
                        ))}
                      </Box>
                    ) : (
                      <Box>{"***"}</Box>
                    )}
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", gap: 2, alignItems: "center" }}>
                      {entry.rtts.history.length > 0 ? (
                        <Fragment>
                          <Box
                            sx={{
                              color: getLatencyColor(entry.rtts.current),
                            }}
                          >
                            {entry.rtts.current.toFixed(3)}ms
                          </Box>
                          <Box
                            sx={{
                              display: "grid",
                              gridTemplateColumns: "repeat(3, auto)",
                              justifyContent: "space-between",
                              justifyItems: "center",
                              alignItems: "center",
                              columnGap: 2,
                            }}
                          >
                            <Fragment>
                              <Box>Min</Box>
                              <Box>Med</Box>
                              <Box>Max</Box>
                              <Box
                                sx={{
                                  color: getLatencyColor(entry.rtts.min),
                                }}
                              >
                                {entry.rtts.min.toFixed(3)}ms
                              </Box>
                              <Box
                                sx={{
                                  color: getLatencyColor(entry.rtts.median),
                                }}
                              >
                                {entry.rtts.median.toFixed(3)}ms
                              </Box>
                              <Box
                                sx={{
                                  color: getLatencyColor(entry.rtts.max),
                                }}
                              >
                                {entry.rtts.max.toFixed(3)}ms
                              </Box>
                            </Fragment>
                          </Box>
                        </Fragment>
                      ) : (
                        <Box>{"***"}</Box>
                      )}
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                      <Box>{entry.stats.sent} Sent,</Box>
                      <Box>{entry.stats.replied} Replied,</Box>
                      <Box>{entry.stats.lost} Lost</Box>
                    </Box>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
    </Fragment>
  );
}
