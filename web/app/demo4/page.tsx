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
import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import { Row, CanvasTable } from "@/components/canvastable";

export type TracerouteReportLocation = {
  city?: string;
  countryAlpha2?: string;
};

export type TracerouteReportISP = {
  // name of the isp, like 'Hurricane Electric'
  ispName: string;

  // usually in the format like 'AS12345'
  asn: string;
};

export type TracerouteReportSource = {
  nodeName: string;
  isp?: TracerouteReportISP;
  loc?: TracerouteReportLocation;
};

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
};

export type TracerouteReportHop = {
  // ttl of the sent packets, usually, when doing traceroute, start with ttl=1,
  // then increment ttl one by one until the final target is reached.
  ttl: number;

  // if the middlebox router is doing something like ecmp, a hop could be various peers.
  peers: TracerouteReportPeer[];
};

export type TracerouteReport = {
  // when the report is generated
  date: number;

  // in case that a originating node is multi-homed/BGP
  sources: TracerouteReport[];

  // the domain or ip address of the target host
  destination: string;

  // type of l4 sending packets, for linux, traceroute use udp by default,
  // for windows, icmp is used.
  mode: TracerouteReportMode;

  hops: TracerouteReportHop[];
};

export function renderTracerouteReport(report: TracerouteReport): {
  preamble: Row[];
  tabularData: Row[];
} {
  const preamble: Row[] = [];
  const tabularData: Row[] = [];

  return { preamble, tabularData };
}

export default function Page() {
  const [show, setShow] = useState(true);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  const preamble: Row[] = useMemo(() => {
    return [
      `Date: ${new Date().toISOString()}`,
      "Source: Node NYC1, AS65001 SOMEISP, SomeCity US",
      "Destination: pingable.burble.dn42",
      "Mode: ICMP",
    ].map((line) => [{ content: line }]);
  }, []);

  const tabularData: Row[] = useMemo(() => {
    return [
      [
        { content: "TTL" },
        { content: "Peers" },
        { content: "ISP" },
        { content: "Location" },
        { content: "RTTs (last min/med/max)" },
        { content: "Stat" },
      ],
      [
        { content: "1" },
        { content: "RFC1819 (192.168.1.1)" },
        { content: "" },
        { content: "" },
        { content: "10ms 1ms/5ms/11ms" },
        { content: "10 sent, 8 replies, 20% loss" },
      ],
      [
        { content: "" },
        { content: "RFC1819 (192.168.2.1)" },
        { content: "" },
        { content: "" },
        { content: "11ms 2ms/6ms/12ms" },
        { content: "9 sent, 7 replies, 22.22% loss" },
      ],
      [{ content: "" }, { content: "*" }],
      [
        { content: "" },
        { content: "10.147.0.1" },
        { content: "" },
        { content: "" },
        { content: "5ms 1ms/4ms/7ms" },
        { content: "8 sent, 7 replies, 12.5% loss" },
      ],
      [
        { content: "2" },
        { content: "h100.1e100.net (123.124.125.126)" },
        { content: "AS65001 [EXAMPLEISP]" },
        { content: "Frankfurt, DE" },
        { content: "10ms 8ms/10ms/11ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "h101.1e100.net (123.124.125.127)" },
        { content: "AS65001 [EXAMPLEISP]" },
        { content: "Frankfurt, DE" },
        { content: "10ms 8ms/10ms/11ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "3" },
        { content: "h100.1e101.net (124.125.126.127)" },
        { content: "AS65002 [EXAMPLEISP2]" },
        { content: "Frankfurt, DE" },
        { content: "12ms 8ms/12ms/14ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "124.125.126.128" },
        { content: "AS65002 [EXAMPLEISP2]" },
        { content: "Frankfurt, DE" },
        { content: "12ms 8ms/12ms/13ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "4" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "4" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "5" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "6" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "7" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "8" },
        { content: "bb1.dod.us(11.1.2.3)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/141ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb2.dod.us(11.1.2.4)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
      [
        { content: "" },
        { content: "bb3.dod.us(11.1.2.5)" },
        { content: "AS65003 [DoD]" },
        { content: "Washington DC, US" },
        { content: "112ms 81ms/121ms/131ms" },
        { content: "8 sent, 7 replies, 12.5 loss" },
      ],
    ];
  }, []);

  return (
    <Box>
      <Button onClick={() => setShow((show) => !show)}>Open</Button>
      <Dialog
        open={show}
        onClose={() => {
          setShow(false);
        }}
        maxWidth={"md"}
        fullWidth
      >
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
                onClick={() => {
                  const canvasEle = canvasRef.current;
                  if (!canvasEle) {
                    console.error("no canvas element found.");
                  }
                  const mimeType = "image/png";
                  canvasEle!.toBlob((blob) => {
                    console.log("[dbg] blob:", blob);
                    if (!blob) {
                      console.error("cant export canvas to blob");
                    }
                    let fname = `trace-${new Date().toISOString()}.png`;
                    fname = fname.replaceAll(" ", "_");
                    const f = new File([blob!], fname, { type: mimeType });
                    const url = URL.createObjectURL(f);
                    const aEle = window.document.createElement("a");
                    aEle.setAttribute("href", url);
                    aEle.setAttribute("download", f.name);
                    aEle.click();
                  }, mimeType);
                }}
              >
                <SaveIcon />
              </IconButton>
            </Tooltip>
          </Box>
        </DialogTitle>
        <DialogContent
          sx={{ paddingLeft: 0, paddingRight: 0, paddingBottom: 0 }}
        >
          <CanvasTable
            preamble={preamble}
            tabularData={tabularData}
            canvasRef={canvasRef as any}
          />
        </DialogContent>
      </Dialog>
    </Box>
  );
}
