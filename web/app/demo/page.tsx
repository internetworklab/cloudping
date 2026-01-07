"use client";

import { useCanvasSizing, WorldMap } from "@/components/worldmap";
import { Box } from "@mui/material";
import { CSSProperties, Fragment, useEffect, useState } from "react";

export default function DemoPage() {
  const [show, setShow] = useState(false);
  //   useEffect(() => {
  //     const ticker = window.setInterval(() => {
  //       setShow((prev) => !prev);
  //     }, 250);
  //     return () => {
  //       window.clearInterval(ticker);
  //     };
  //   }, []);

  const canvasWidth = 40000;
  const canvasHeight = 25000;

  const { canvasSvgRef } = useCanvasSizing(canvasWidth, canvasHeight, show);
  const fill: CSSProperties["fill"] = "hsl(202deg 32% 50%)";

  return (
    <Box
      sx={{
        width: "100vw",
        height: "100vh",
        position: "fixed",
        top: 0,
        left: 0,
        overflow: "hidden",
        backgroundColor: "#242424",
      }}
    >
      <WorldMap
        canvasSvgRef={canvasSvgRef as any}
        canvasWidth={canvasWidth}
        canvasHeight={canvasHeight}
        fill={fill}
        markers={[]}
      />
    </Box>
  );
}
