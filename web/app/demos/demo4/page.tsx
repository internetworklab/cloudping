"use client";

import { useEffect, useRef, useState } from "react";
import { Box } from "@mui/material";
import { CanvasTable } from "@/components/canvastable";

const preamble = [
  [{ content: "PING dns.google (8.8.8.8) 56(84) bytes of data." }],
  [{ content: "" }],
];

const initialTabularData = [
  [
    { content: "Seq" },
    { content: "Reply" },
    { content: "Size" },
    { content: "Time" },
    { content: "TTL" },
  ],
  [
    { content: "1" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.3 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "2" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=9.8 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "3" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=11.1 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "4" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.7 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "5" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.0 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "6" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=12.5 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "7" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=9.4 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "8" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.9 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "9" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=11.4 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "10" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.2 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "11" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.2 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "12" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.2 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "13" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.2 ms" },
    { content: "TTL=118" },
  ],
  [
    { content: "14" },
    { content: "Reply from dns.google (8.8.8.8)" },
    { content: "bytes=64" },
    { content: "time=10.2 ms" },
    { content: "TTL=118" },
  ],
];

export default function Page() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [tabularData, setTabularData] = useState(initialTabularData);
  const seqRef = useRef(initialTabularData.length - 1);

  useEffect(() => {
    const id = setInterval(() => {
      seqRef.current += 1;
      const latency = (Math.random() * 4 + 9).toFixed(1);
      setTabularData((prev) => [
        ...prev,
        [
          { content: String(seqRef.current) },
          { content: "Reply from dns.google (8.8.8.8)" },
          { content: "bytes=64" },
          { content: `time=${latency} ms` },
          { content: "TTL=118" },
        ],
      ]);
    }, 1000);

    return () => clearInterval(id);
  }, []);

  return (
    <Box sx={{ height: "80vh", width: "80vw", overflow: "hidden" }}>
      <CanvasTable
        preamble={preamble}
        tabularData={tabularData}
        canvasRef={canvasRef}
      />
    </Box>
  );
}
