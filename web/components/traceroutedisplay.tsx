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
  Card,
  Tooltip,
  IconButton,
} from "@mui/material";
import {
  CSSProperties,
  Fragment,
  ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";
import { TaskCloseIconButton } from "@/components/taskclose";

import { getLatencyColor } from "./colorfunc";
import { TraceEvent, TraceStats } from "@/apis/trace";
import {
  ConnEntry,
  getNodes,
  getPingSampleStreamURL,
  JSONLineDecoder,
  LineTokenizer,
  NodeAttrASN,
  NodeAttrCityName,
  NodeAttrCountryCode,
  NodeAttrDN42ASN,
  NodeAttrDN42ISP,
  NodeAttrISP,
} from "@/apis/globalping";
import { PendingTask, RawPingEvent, RawPingEventData } from "@/apis/types";
import {
  LonLat,
  Marker,
  Path,
  toGeodesicPaths,
  useCanvasSizing,
  useZoomControl,
  WorldMap,
  ZoomHintText,
} from "./worldmap";
import MapIcon from "@mui/icons-material/Map";
import { useQuery } from "@tanstack/react-query";
import { getNodeGroups } from "@/apis/utils";
import {
  TracerouteReport,
  TracerouteReportMode,
  TracerouteReportPreviewDialog,
} from "@/components/traceroutereport";
import ShareIcon from "@mui/icons-material/Share";
import { firstLetterCap } from "./strings";
import { defaultResolver } from "@/apis/resolver";
import RefreshIcon from "@mui/icons-material/Refresh";
import PlayIcon from "@mui/icons-material/PlayArrow";
import StopIcon from "@mui/icons-material/Stop";

// RttCell parses RTT strings like "3ms 1ms/4ms/20ms" and renders each value
// with a color based on its latency using getLatencyColor.
function RttCell({ value }: { value: string }) {
  if (!value) {
    return null;
  }

  if (value === "* */*/*") {
    return <>{value}</>;
  }

  const spaceIdx = value.indexOf(" ");
  if (spaceIdx === -1) {
    return <>{value}</>;
  }

  const lastRttStr = value.substring(0, spaceIdx);
  const minAvgMaxStr = value.substring(spaceIdx + 1);

  const parseMs = (s: string): number | null => {
    const match = s.match(/^(\d+(?:\.\d+)?)ms$/);
    return match ? parseFloat(match[1]) : null;
  };

  const lastRtt = parseMs(lastRttStr);
  const parts = minAvgMaxStr.split("/");

  return (
    <Box component="span">
      <Box component="span" sx={{ color: getLatencyColor(lastRtt) }}>
        {lastRttStr}
      </Box>{" "}
      {parts.map((part, i) => (
        <Fragment key={i}>
          {i > 0 && "/"}
          <Box component="span" sx={{ color: getLatencyColor(parseMs(part)) }}>
            {part}
          </Box>
        </Fragment>
      ))}
    </Box>
  );
}

const worldMapFill: CSSProperties["fill"] = "#676767";

function DisplayCurrentNode(props: {
  currentNode: ConnEntry | undefined;
  target: string;
}) {
  const { currentNode, target } = props;
  if (!currentNode) {
    return <Fragment></Fragment>;
  }

  const city: string | undefined = currentNode?.attributes?.[NodeAttrCityName];
  const countryAlpha2: string | undefined =
    currentNode?.attributes?.[NodeAttrCountryCode];
  const ispASN: string | undefined = currentNode?.attributes?.[NodeAttrASN];
  const ispName: string | undefined = currentNode?.attributes?.[NodeAttrISP];
  const dn42ISPASN: string | undefined =
    currentNode?.attributes?.[NodeAttrDN42ASN];
  const dn42ISPName: string | undefined =
    currentNode?.attributes?.[NodeAttrDN42ISP];

  const nodeDisplayName = currentNode.node_name?.toUpperCase();

  const nodeInfos: string[][] = [];
  if (nodeDisplayName) {
    nodeInfos.push(["Node", nodeDisplayName]);
  }
  if (ispASN) {
    nodeInfos.push(ispName ? [ispASN, ispName] : [ispASN]);
  }
  if (countryAlpha2) {
    nodeInfos.push(city ? [city, countryAlpha2] : [countryAlpha2]);
  }
  if (dn42ISPASN) {
    nodeInfos.push(
      dn42ISPName ? ["DN42", dn42ISPASN, dn42ISPName] : ["DN42", dn42ISPASN],
    );
  }

  return (
    <Box sx={{ padding: 2 }}>
      <Typography variant="body2">
        From:{" "}
        {nodeInfos.map((word) => word.filter((s) => !!s).join(" ")).join(", ")}
      </Typography>
      <Typography variant="body2">To: {target}</Typography>
    </Box>
  );
}

function getMarkersAndPaths(
  traceStats: TraceStats | undefined,
  showMap: boolean,
  sourceMarkers: Marker[],
  ratio: number,
): { markers: Marker[]; extraPaths: Path[] | undefined } {
  if (!showMap || !traceStats) {
    return { markers: [], extraPaths: undefined };
  }

  const sampleMarkers: Marker[] = [];
  for (const hopTTL of traceStats.HopOrder) {
    const hop = traceStats.Hops.get(hopTTL);
    if (!hop) continue;

    for (const peerKey of hop.PeerOrder) {
      const peerStats = hop.Peers.get(peerKey);
      if (!peerStats) continue;
      if (peerStats.ReceivedCount === 0) continue;

      // Find location from the first non-timeout event
      let exactLoc: { Longitude: number; Latitude: number } | null = null;
      for (const ev of peerStats.Events) {
        if (ev.ExactLocation && !ev.Timeout) {
          exactLoc = ev.ExactLocation;
          break;
        }
      }
      if (!exactLoc) continue;

      const lonLat: LonLat = [exactLoc.Longitude, exactLoc.Latitude];
      const rtt = peerStats.AvgRTT();
      const fill = getLatencyColor(rtt);
      const index = `TTL=${hopTTL}, IP=${peerStats.Peer}`;
      const tooltip: ReactNode = (
        <Box>
          <Box>TTL:&nbsp;{hopTTL}</Box>
          <Box>IP:&nbsp;{peerStats.Peer}</Box>
          {peerStats.PeerRDNS && <Box>RDNS:&nbsp;{peerStats.PeerRDNS}</Box>}
        </Box>
      );

      sampleMarkers.push({
        lonLat,
        fill,
        radius: 8,
        strokeWidth: 3,
        stroke: "white" as CSSProperties["stroke"],
        tooltip,
        index,
        metadata: { ttl: hopTTL },
      });
    }
  }

  // Build paths between consecutive sample markers
  const samplePaths: Path[] = [];
  if (sampleMarkers.length > 1) {
    for (let j = 1; j < sampleMarkers.length; j++) {
      const fromMarker = sampleMarkers[j - 1];
      const toMarker = sampleMarkers[j];
      if (fromMarker && toMarker && fromMarker.lonLat && toMarker.lonLat) {
        const paths = toGeodesicPaths(
          [fromMarker.lonLat[1], fromMarker.lonLat[0]],
          [toMarker.lonLat[1], toMarker.lonLat[0]],
          200,
        );
        for (const path of paths) {
          samplePaths.push({
            ...path,
            stroke: "green",
            strokeWidth: 4 * ratio,
          });
        }
      }
    }
  }

  // Prepend source markers and apply ratio scaling to all markers
  const markers = [...sourceMarkers, ...sampleMarkers].map((m) => ({
    ...m,
    radius: m.radius ? m.radius * ratio : undefined,
    strokeWidth: m.strokeWidth ? m.strokeWidth * ratio : undefined,
  }));

  // Build source-to-first-hop path
  const srcPaths: Path[] = [];
  if (sourceMarkers.length > 0 && sampleMarkers.length > 0) {
    const src = sourceMarkers[0];
    const first = sampleMarkers[0];
    if (src.lonLat && first.lonLat) {
      const paths = toGeodesicPaths(
        [src.lonLat[1], src.lonLat[0]],
        [first.lonLat[1], first.lonLat[0]],
        200,
      );
      for (const path of paths) {
        srcPaths.push({
          ...path,
          stroke: "green",
          strokeWidth: 4 * ratio,
        });
      }
    }
  }

  const extraPaths: Path[] | undefined =
    srcPaths.length > 0 || samplePaths.length > 0
      ? [...srcPaths, ...samplePaths]
      : undefined;

  return { markers, extraPaths };
}

function buildTracerouteReport(
  from: ConnEntry,
  target: string,
  mode: TracerouteReportMode,
): TracerouteReport {
  const now = new Date();

  const isp = from?.attributes?.[NodeAttrISP];
  let asn = from?.attributes?.[NodeAttrASN];
  if (asn && isp) {
    asn = asn + " " + isp;
  }

  let dn42ASN = from?.attributes?.[NodeAttrDN42ASN];
  const dn42ISP = from?.attributes?.[NodeAttrDN42ISP];
  if (dn42ASN || dn42ISP) {
    dn42ASN = dn42ASN + " " + dn42ISP;
  }

  const report: TracerouteReport = {
    date: now.valueOf(),
    sources: [
      {
        nodeName: from.node_name?.toUpperCase() || "unknown",
        isp: {
          ispName: "",
          asn: "",
          network: [asn, dn42ASN].filter((s) => !!s).join(" | "),
        },
        loc: {
          city: from?.attributes?.[NodeAttrCityName] || "unknown",
          countryAlpha2: from?.attributes?.[NodeAttrCountryCode] || "unknown",
        },
      },
    ],
    destination: target,
    mode: mode,
  };

  return report;
}

export class TraceEventAdapter extends TransformStream<
  unknown,
  [RawPingEvent<RawPingEventData>, TraceEvent]
> {
  constructor() {
    super({
      transform(
        chunk: unknown,
        controller: TransformStreamDefaultController<unknown>,
      ) {
        const rawEventObj = chunk as RawPingEvent<RawPingEventData>;
        if (!rawEventObj) {
          console.error("skipping nil raw ping event:", rawEventObj);
          return;
        }
        const evObj = TraceEvent.fromRaw(rawEventObj);
        if (evObj) {
          controller.enqueue([rawEventObj, evObj]);
        } else {
          console.error("Ignore raw ping event:", rawEventObj);
        }
      },
    });
  }
}

function useTraceEventsRead(task: PendingTask): {
  traceStatsMap: Record<string, TraceStats>;
  stopTick: () => void;
  resumeTick: () => void;
  isRunning: boolean;
  paused: boolean;
} {
  const [traceStatsMap, setTraceStatsMap] = useState<
    Record<string, TraceStats>
  >({});
  const [snapshop, setSnapshop] = useState<
    Record<string, TraceStats> | undefined
  >(undefined);

  // isRunning is for indicating whether it's actually running
  const [isRunning, setIsRunning] = useState(false);

  // paused is for inicating whether is's been manually paused
  const [paused, setPaused] = useState(false);

  const resumeTick = (abortController: AbortController, task: PendingTask) => {
    const url = getPingSampleStreamURL({
      sources: task.sources,
      targets: task.targets.slice(0, 1),
      intervalMs: 300,
      pktTimeoutMs: 3000,
      ttl: "auto",
      resolver: defaultResolver,
      ipInfoProviderName: "auto",
      preferV4: task.preferV4,
      preferV6: task.preferV6,
      l4PacketType: !!task.useUDP ? "udp" : "icmp",
      randomPayloadSize: task.pmtu ? 1500 : undefined,
    });

    fetch(url, { signal: abortController.signal })
      .then((r) => r.body)
      .then((r) => {
        return r
          ?.pipeThrough(new TextDecoderStream())
          .pipeThrough(new LineTokenizer())
          .pipeThrough(new JSONLineDecoder())
          .pipeThrough(new TraceEventAdapter())
          .getReader();
      })
      .then(async (reader) => {
        if (!reader) {
          return;
        }
        setIsRunning(true);
        while (true) {
          try {
            const { value, done } = await reader.read();
            if (done) {
              setIsRunning(false);
              return;
            }

            const raw = value[0];
            const ev = value[1];
            const from = raw.metadata?.from;
            if (from) {
              setTraceStatsMap((prev) => {
                const stat = prev[from] ?? new TraceStats();
                return { ...prev, [from]: stat.WriteEvent(ev) };
              });
            }
          } catch (err) {
            console.error("failed to read:", err);
            return;
          }
        }
      })
      .catch((err) => {
        if (err.name === "AbortError") {
          console.log("Stream stopped by user or component unmount.");
        } else {
          console.error("Stream error:", err);
        }
      });
  };
  const abortControllerRef = useRef<AbortController>(new AbortController());

  useEffect(() => {
    if (paused) {
      return;
    }

    const abortController = new AbortController();
    resumeTick(abortController, task);
    abortControllerRef.current = abortController;

    return () => {
      abortController.abort();
      setIsRunning(false);
      setTraceStatsMap({});
    };
  }, [task, paused]);

  return {
    traceStatsMap: snapshop || traceStatsMap,
    isRunning,
    stopTick: () => {
      setSnapshop(traceStatsMap);
      setPaused(true);
      abortControllerRef.current?.abort();
    },
    resumeTick: () => {
      setPaused(false);
      setSnapshop(undefined);
    },
    paused,
  };
}

export function TracerouteResultDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
}) {
  const { task, onDeleted } = props;

  const [tabValue, setTabValue] = useState(task.sources[0]);

  const [report, setReport] = useState<TracerouteReport | undefined>(undefined);

  const canvasW = 360000;
  const canvasH = 200000;

  const [showMap, setShowMap] = useState<boolean>(false);

  const { zoomEnabled } = useZoomControl();

  const { canvasSvgRef, ratio = 1 } = useCanvasSizing(
    canvasW,
    canvasH,
    showMap,
    zoomEnabled,
  );

  const { data: conns } = useQuery({
    queryKey: ["nodes"],
    queryFn: () => getNodes(),
  });
  const sourceSet = new Set<string>([tabValue]);
  const nodeGroups = getNodeGroups(conns || {}, sourceSet);
  const sourceMarkers: Marker[] = [];
  if (nodeGroups && nodeGroups.length > 0) {
    const node = nodeGroups[0];
    if (node && node.latLon) {
      sourceMarkers.push({
        lonLat: [node.latLon[1], node.latLon[0]],
        fill: "blue",
        radius: 8,
        strokeWidth: 3,
        stroke: "white",
        index: `(SRC)`,
      });
    }
  }

  const { traceStatsMap, isRunning, stopTick, resumeTick, paused } =
    useTraceEventsRead(task);

  const traceStats = traceStatsMap[tabValue];
  const tracerouteTable = traceStats?.ToTable();

  const currentNode = Object.values(conns || {}).find(
    (entry) => entry.node_name === tabValue,
  );

  const [openPreview, setOpenPreview] = useState(false);
  const { markers, extraPaths } = getMarkersAndPaths(
    traceStats,
    showMap,
    sourceMarkers,
    ratio,
  );

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
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <Typography variant="h6">
            {firstLetterCap(task.type)} Task #{task.taskId}
          </Typography>
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
          <Tooltip title={showMap ? "Hide Map" : "Show Map"}>
            <IconButton
              sx={{
                visibility:
                  markers && markers.length > 0 ? "visible" : "hidden",
              }}
              onClick={() => setShowMap(!showMap)}
            >
              <MapIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title="Share Report">
            <IconButton
              onClick={() => {
                const from = currentNode;
                const target = task?.targets?.at(0) || "";
                if (from && target && traceStats) {
                  setReport(
                    buildTracerouteReport(
                      from,
                      target,
                      !!task?.useUDP ? "udp" : "icmp",
                    ),
                  );
                  setOpenPreview(true);
                }
              }}
            >
              <ShareIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title={isRunning ? "Stop" : "Resume"}>
            <IconButton
              disabled={(paused && isRunning) || (!paused && !isRunning)}
              onClick={() => {
                if (isRunning) {
                  stopTick();
                } else {
                  resumeTick();
                }
              }}
            >
              {isRunning ? <StopIcon /> : <PlayIcon />}
            </IconButton>
          </Tooltip>

          <TaskCloseIconButton
            taskId={task.taskId}
            onConfirmedClosed={() => {
              onDeleted();
            }}
          />
        </Box>
      </Box>

      <DisplayCurrentNode
        currentNode={currentNode}
        target={task?.targets?.at(0) || ""}
      />

      {showMap && (
        <Box
          sx={{
            height: showMap ? "360px" : "36px",
            position: "relative",
            top: 0,
            left: 0,
          }}
        >
          <WorldMap
            canvasSvgRef={canvasSvgRef}
            canvasWidth={canvasW}
            canvasHeight={canvasH}
            fill={worldMapFill}
            paths={extraPaths}
            markers={markers}
          />

          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              flexWrap: "wrap",
              gap: 2,
              position: "absolute",
              bottom: 0,
              alignItems: "center",
              padding: 2,
              width: "100%",
              fontSize: 12,
            }}
          >
            Traceroute to {task.targets[0]}, for informational purposes only.
          </Box>

          {!zoomEnabled && (
            <Box
              sx={{
                position: "absolute",
                top: 2,
                left: 2,
                fontSize: 12,
                padding: 2,
              }}
            >
              <ZoomHintText />
            </Box>
          )}
        </Box>
      )}

      <TableContainer sx={{ maxWidth: "100%", overflowX: "auto" }}>
        <Table>
          <TableHead>
            {tracerouteTable?.header?.map((row, rowIdx, rows) => (
              <TableRow key={rowIdx}>
                {row.cells.map((cell, cellIdx) => (
                  <TableCell
                    key={cellIdx}
                    sx={{
                      fontWeight: "bold",
                      borderBottom: rowIdx === 0 ? "none" : undefined,
                      padding: 0,
                      paddingLeft: 1,
                      paddingRight: 1,
                      paddingTop: rowIdx === 0 ? 2 : undefined,
                      paddingBottom: rowIdx === rows.length - 1 ? 2 : undefined,
                    }}
                  >
                    {cell}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableHead>
          <TableBody>
            {tracerouteTable?.rows?.map((row, rowIdx) => {
              if (row.spacer) {
                return <Fragment key={rowIdx}></Fragment>;
              }
              const rows = tracerouteTable?.rows || [];
              const hopFirstRow = !!row.cells[0];
              const isLastOfHop =
                rowIdx === rows.length - 1 || rows[rowIdx + 1]?.spacer;
              return (
                <TableRow key={rowIdx}>
                  {row.cells.map((cell, cellIdx) => (
                    <TableCell
                      key={cellIdx}
                      sx={{
                        padding: 0,
                        paddingLeft: 1,
                        paddingRight: 1,
                        paddingTop: hopFirstRow ? 2 : undefined,
                        paddingBottom: isLastOfHop ? 2 : undefined,
                        whiteSpace: "nowrap",
                        borderBottom: isLastOfHop ? undefined : "none",
                      }}
                    >
                      {hopFirstRow && cellIdx === 2 ? (
                        <RttCell value={cell} />
                      ) : (
                        cell
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
      <TracerouteReportPreviewDialog
        report={report}
        tracerouteTable={tracerouteTable}
        open={!!(report && tracerouteTable && openPreview)}
        onClose={() => {
          setOpenPreview(false);
        }}
      />
    </Card>
  );
}
