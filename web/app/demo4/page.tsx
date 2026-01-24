"use client";

import { Box, Button, Dialog, DialogContent, DialogTitle } from "@mui/material";
import { Fragment, useEffect, useRef, useState } from "react";

type LineDrawCtx = {
  fillStyle?: CanvasPattern | string | CanvasGradient;
  baseline?: CanvasTextBaseline;
  font?: string;
  y: number;
  lineGap: number;
  lastMetrics?: TextMetrics;
  x: number;
  maxWidth?: number;
};

function drawLine(
  lineCtx: LineDrawCtx,
  ctx: CanvasRenderingContext2D,
  line: string
): LineDrawCtx {
  if (lineCtx.baseline) {
    ctx.textBaseline = lineCtx.baseline;
  }
  if (lineCtx.fillStyle) {
    ctx.fillStyle = lineCtx.fillStyle;
  }
  if (lineCtx.font) {
    ctx.font = lineCtx.font;
  }
  ctx.fillText(line, lineCtx.x, lineCtx.y);
  const lineMs = ctx.measureText(line);
  const prevMaxWidth = lineCtx.maxWidth ?? 0;
  return {
    ...lineCtx,
    y:
      lineCtx.y +
      lineMs.actualBoundingBoxAscent +
      lineMs.actualBoundingBoxDescent +
      lineCtx.lineGap,
    lastMetrics: lineMs,
    maxWidth: Math.max(prevMaxWidth, lineMs.width),
  };
}

function drawCursor(
  x: number,
  y: number,
  ctx: CanvasRenderingContext2D,
  w: number,
  h: number,
  color: CanvasPattern | string | CanvasGradient
): void {
  const currentFill = ctx.fillStyle;
  ctx.fillStyle = color;
  ctx.fillRect(x, y, w, h);
  ctx.fillStyle = currentFill;
}

type ColDrawCtx = {
  lineDrawCtx: LineDrawCtx;
  columnGap: number;
  y0?: number;
};

function drawCol(
  colCtx: ColDrawCtx,
  ctx: CanvasRenderingContext2D,
  column: string[]
): ColDrawCtx {
  let newColCtx: ColDrawCtx = {
    ...colCtx,
    y0: colCtx.y0 ?? colCtx.lineDrawCtx.y,
  };
  newColCtx.lineDrawCtx = {
    ...newColCtx.lineDrawCtx,
    maxWidth: 0,
    y: newColCtx.y0!,
  };

  for (const row of column) {
    newColCtx.lineDrawCtx = drawLine(newColCtx.lineDrawCtx, ctx, row);
  }

  newColCtx.lineDrawCtx = {
    ...newColCtx.lineDrawCtx,
    x:
      newColCtx.lineDrawCtx.x +
      (newColCtx.lineDrawCtx.maxWidth ?? 0) +
      newColCtx.columnGap,
    maxWidth: 0,
  };

  return newColCtx;
}

function carriageReturn(lineCtx: LineDrawCtx): LineDrawCtx {
  return {
    ...lineCtx,
    x: 0,
  };
}

function Window() {
  const [w, setW] = useState(0);
  const [h, setH] = useState(0);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const ele = boxRef.current;
    if (!ele) {
      return;
    }

    const rect = ele.getBoundingClientRect();
    console.log("[dbg] rect:", rect);

    const obs = new ResizeObserver((entries) => {
      for (const ent of entries) {
        if (ent.target === ele) {
          console.log("[dbg] ResizeObserver entry:", ent);
          const cr = ent.contentRect;
          if (cr) {
            setW(cr.width);
            setH(cr.height);
          }
        }
      }
    });

    obs.observe(ele);

    return () => {
      obs.unobserve(ele);
    };
  });

  useEffect(() => {
    const ele = boxRef.current;
    if (!ele) {
      return;
    }

    const canvasEle = ele.querySelector("canvas");
    if (!canvasEle) {
      return;
    }

    const dpi = window.devicePixelRatio;
    canvasEle.setAttribute("width", `${w * dpi}`);
    canvasEle.setAttribute("height", `${h * dpi}`);

    const ctx = canvasEle.getContext("2d");
    if (!ctx) {
      return;
    }

    ctx.fillStyle = "#262626";
    ctx.fillRect(0, 0, w * dpi, h * dpi);

    const lines: string[] = [
      `Date: ${new Date().toISOString()}`,
      "Source: Node NYC1, AS65001 SOMEISP",
      "Destination: pingable.burble.dn42",
    ];
    let lineCtx: LineDrawCtx = {
      fillStyle: "#fff",
      baseline: "top",
      font: `${16 * dpi}px sans-serif`,
      y: 0,
      lineGap: 10 * dpi,
      x: 0,
    };

    for (const line of lines) {
      lineCtx = drawLine(lineCtx, ctx, line);
    }

    // drawCursor(lineCtx, ctx, 15 * dpi, 20 * dpi, "#111");

    const col1 = ["col1", "A", "A1111", "A11", "A1111111", "A1"];

    let colCtx: ColDrawCtx = {
      lineDrawCtx: lineCtx,
      columnGap: 10 * dpi,
    };
    colCtx = drawCol(colCtx, ctx, col1);

    const col2 = ["col2", "B", "B1111", "B11", "B11111", "B111"];
    colCtx = drawCol(colCtx, ctx, col2);

    const col3 = ["col3", "C", "C1111", "C11", "C11111", "C111"];
    colCtx = drawCol(colCtx, ctx, col3);

    colCtx = {
      ...colCtx,
      lineDrawCtx: carriageReturn(colCtx.lineDrawCtx),
    };

    drawCursor(
      colCtx.lineDrawCtx.x,
      colCtx.lineDrawCtx.y,
      ctx,
      15 * dpi,
      20 * dpi,
      "#111"
    );

    console.log("[dbg] paint.");
  });

  return (
    <Fragment>
      <Box
        ref={boxRef}
        sx={{
          height: "400px",
        }}
      >
        <Box sx={{ width: "100%", height: "100%" }} component={"canvas"}></Box>
      </Box>
      <Box sx={{ paddingTop: 2, paddingLeft: 3, paddingRight: 3 }}>
        <Box>W: {w}</Box>
        <Box>H: {h}</Box>
      </Box>
    </Fragment>
  );
}

export default function Page() {
  const [show, setShow] = useState(true);

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
        <DialogTitle>Preview</DialogTitle>
        <DialogContent sx={{ paddingLeft: 0, paddingRight: 0 }}>
          <Window />
        </DialogContent>
      </Dialog>
    </Box>
  );
}
