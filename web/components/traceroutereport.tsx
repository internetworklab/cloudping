"use client";

import {
  Box,
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Tooltip,
} from "@mui/material";
import { useMemo, useRef, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import { Row, CanvasTable } from "@/components/canvastable";
import { Table } from "@/apis/trace";

export type TracerouteReportLocation = {
  city?: string;
  countryAlpha2?: string;
};

function formatNumStr(num: string): string {
  return num.replace(/0+$/, "").replace(/\.$/, "");
}

function renderLoc(loc?: TracerouteReportLocation): string {
  if (!loc) {
    return "";
  }
  const locLine: string[] = [];
  if (loc.city) {
    locLine.push(loc.city);
  }
  if (loc.countryAlpha2) {
    locLine.push(loc.countryAlpha2);
  }
  if (locLine.length === 0) {
    return "";
  }
  return locLine.join(", ");
}

export type TracerouteReportISP = {
  // name of the isp, like 'Hurricane Electric'
  ispName: string;

  // usually in the format like 'AS12345'
  asn: string;

  // take precedence, display asn and network all together
  network?: string;
};

function renderISP(isp?: TracerouteReportISP): string {
  if (isp?.network) {
    return isp.network;
  }
  if (!isp) {
    return "";
  }
  const ispLine: string[] = [];
  if (isp.asn) {
    ispLine.push(isp.asn);
  }
  if (isp.ispName) {
    ispLine.push(isp.ispName);
  }
  if (ispLine.length === 0) {
    return "";
  }
  return ispLine.join(" ");
}

export type TracerouteReportSource = {
  nodeName: string;
  isp?: TracerouteReportISP;
  loc?: TracerouteReportLocation;
};

function renderSource(src?: TracerouteReportSource): string {
  if (!src) {
    return "";
  }

  if (!src.nodeName) {
    return "";
  }

  const srcLine: string[] = [];
  srcLine.push("Node " + src.nodeName);
  const ispLine = renderISP(src.isp);
  if (ispLine) {
    srcLine.push(ispLine);
  }
  const locLine = renderLoc(src.loc);
  if (locLine) {
    srcLine.push(locLine);
  }
  return srcLine.join(", ");
}

export type TracerouteReportMode = "tcp" | "icmp" | "udp";

export type TracerouteReportRTTStat = {
  lastMs: number;
  samples: number[];
};

export type TracerouteReportTXRXStat = {
  sent: number;
  replies: number;
};

export type TracerouteReportPeer = {
  // if this field is falsy, mark it with a '*' in the screen,
  // and skip render all the following fields.
  timeout?: boolean;
  rdns?: string;
  ip: string;
  loc?: TracerouteReportLocation;
  isp?: TracerouteReportISP;
  rtt?: TracerouteReportRTTStat;
  stat?: TracerouteReportTXRXStat;
  pmtu?: number;
};

export type TracerouteReportHop = {
  // ttl of the sent packets, usually, when doing traceroute, start with ttl=1,
  // then increment ttl one by one until the final target is reached.
  ttl: number;

  // if the middlebox router is doing something like ecmp, a hop could be various peers.
  peers: TracerouteReportPeer[];
};

// Note: this struct is solely for storing metadata of a traceroute task.
export type TracerouteReport = {
  // when the report is generated
  date: number;

  // in case that a originating node is multi-homed/BGP
  sources: TracerouteReportSource[];

  // the domain or ip address of the target host
  destination: string;

  // type of l4 sending packets, for linux, traceroute use udp by default,
  // for windows, icmp is used.
  mode: TracerouteReportMode;
};

function renderTracerouteReport(
  report: TracerouteReport,
  tracerouteTable: Table,
): {
  preamble: Row[];
  tabularData: Row[];
} {
  const preamble: Row[] = [];

  if (report.date !== 0) {
    preamble.push([
      { content: `Date: ${new Date(report.date).toISOString()}` },
    ]);
  }
  for (const src of report.sources) {
    const line = renderSource(src);
    if (line) {
      preamble.push([{ content: "Source: " + line }]);
    }
  }

  if (report.destination) {
    preamble.push([{ content: "Destination: " + report.destination }]);
  }

  if (report.mode) {
    preamble.push([{ content: "Mode: " + report.mode.toUpperCase() }]);
  }

  const tabularData: Row[] = [];

  // Render tabular data from tracerouteTable
  for (const headerRow of tracerouteTable.header) {
    tabularData.push(headerRow.cells.map((cell) => ({ content: cell })));
  }

  // Spacer between header and body
  tabularData.push([{ content: "", empty: true }]);

  for (const row of tracerouteTable.rows) {
    if (row.spacer) {
      tabularData.push([{ content: "", empty: true }]);
    } else {
      tabularData.push(row.cells.map((cell) => ({ content: cell })));
    }
  }

  return { preamble, tabularData };
}

export type PingReport = {
  date: number;
  mode: TracerouteReportMode;
  sources: string[];
  targets: string[];

  preferV4?: boolean;
  preferV6?: boolean;

  // to -> from -> rtt
  rtts: Record<string, Record<string, number>>;
};

function renderPingReport(report: PingReport): {
  preamble: Row[];
  tabularData: Row[];
} {
  const preamble: Row[] = [];
  const tabularData: Row[] = [];

  if (report.date !== 0) {
    preamble.push([
      { content: `Date: ${new Date(report.date).toISOString()}` },
    ]);
  }

  if (report.mode) {
    preamble.push([{ content: "Mode: " + report.mode.toUpperCase() }]);
  }

  if (!!report.preferV4) {
    preamble.push([{ content: "Prefer IPv4: true" }]);
  }

  if (!!report.preferV6) {
    preamble.push([{ content: "Prefer IPv6: true" }]);
  }

  if (report.sources && report.sources.length > 0) {
    const header: Row = [{ content: "Tgt\\Src" }];
    for (const src of report.sources) {
      header.push({ content: src?.toUpperCase() || "" });
    }
    tabularData.push(header);

    if (report.targets && report.targets.length > 0) {
      for (const tgt of report.targets) {
        const tgtRow: Row = [{ content: tgt }];

        for (const src of report.sources) {
          const rtt = report.rtts?.[tgt]?.[src];
          if (rtt !== undefined && rtt !== null) {
            tgtRow.push({ content: `${formatNumStr(rtt.toFixed(2))}ms` });
          }
        }

        tabularData.push(tgtRow);
      }
    }
  }

  return { preamble, tabularData };
}

function exportCanvasBitmap(
  canvasRef: React.RefObject<HTMLCanvasElement | null>,
  fname: string,
): Promise<void> {
  const canvasEle = canvasRef.current;
  if (!canvasEle) {
    console.error("no canvas element found.");
  }
  const mimeType = "image/png";
  fname = fname.replaceAll(" ", "_");

  return new Promise<void>((resolve) => {
    canvasEle!.toBlob((blob) => {
      if (!blob) {
        console.error("cant export canvas to blob");
      }

      const f = new File([blob!], fname, { type: mimeType });
      const url = URL.createObjectURL(f);
      const aEle = window.document.createElement("a");
      aEle.setAttribute("href", url);
      aEle.setAttribute("download", f.name);
      aEle.click();
      resolve();
    }, mimeType);
  });
}

export function TracerouteReportPreviewDialog(props: {
  report: TracerouteReport | undefined;
  tracerouteTable: Table | undefined;
  open: boolean;
  onClose: () => void;
}) {
  const { report, tracerouteTable, open, onClose } = props;

  const canvasRef = useRef<HTMLCanvasElement>(null);

  const { preamble, tabularData } = useMemo(() => {
    if (report && tracerouteTable) {
      return renderTracerouteReport(report, tracerouteTable);
    }
    return { preamble: [], tabularData: [] };
  }, [report, tracerouteTable]);

  const [exporting, setExporting] = useState(false);

  return (
    <Dialog open={open} onClose={onClose} maxWidth={"md"} fullWidth>
      <DialogTitle>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: 2,
            alignItems: "center",
          }}
        >
          Preview
          <Tooltip title="Save to disk">
            <IconButton
              loading={exporting}
              onClick={() => {
                const date = report?.date;
                if (date !== undefined && date !== null) {
                  const fname = `trace-${new Date(date).toISOString()}.png`;
                  setExporting(true);
                  exportCanvasBitmap(canvasRef, fname).finally(() =>
                    setExporting(false),
                  );
                }
              }}
            >
              <SaveIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </DialogTitle>
      <DialogContent sx={{ paddingLeft: 0, paddingRight: 0, paddingBottom: 0 }}>
        <CanvasTable
          preamble={preamble}
          tabularData={tabularData}
          canvasRef={canvasRef}
        />
      </DialogContent>
    </Dialog>
  );
}

export function PingReportPreviewDialog(props: {
  report: PingReport | undefined;
  open: boolean;
  onClose: () => void;
}) {
  const { report, open, onClose } = props;

  const canvasRef = useRef<HTMLCanvasElement>(null);

  const { preamble, tabularData } = useMemo(() => {
    if (report) {
      return renderPingReport(report);
    }
    return { preamble: [], tabularData: [] };
  }, [report]);

  const [exporting, setExporting] = useState(false);

  return (
    <Dialog open={open} onClose={onClose} maxWidth={"md"} fullWidth>
      <DialogTitle>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: 2,
            alignItems: "center",
          }}
        >
          Preview
          <Tooltip title="Save to disk">
            <IconButton
              loading={exporting}
              onClick={() => {
                const date = report?.date;
                if (date !== undefined && date !== null) {
                  const fname = `ping-${new Date(date).toISOString()}.png`;
                  setExporting(true);
                  exportCanvasBitmap(canvasRef, fname).finally(() =>
                    setExporting(false),
                  );
                }
              }}
            >
              <SaveIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </DialogTitle>
      <DialogContent sx={{ paddingLeft: 0, paddingRight: 0, paddingBottom: 0 }}>
        <CanvasTable
          preamble={preamble}
          tabularData={tabularData}
          canvasRef={canvasRef}
        />
      </DialogContent>
    </Dialog>
  );
}
