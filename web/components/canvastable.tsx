"use client";

import { Box } from "@mui/material";
import { Fragment, RefObject, useEffect, useRef } from "react";

type LineDrawCtx = {
  fillStyle?: CanvasPattern | string | CanvasGradient;
  baseline?: CanvasTextBaseline;
  font?: string;
  y: number;
  lineGap: number;
  lastMetrics?: TextMetrics;
  x: number;
  maxWidth?: number;
  maxRight?: number;
  maxHeight?: number;
};

function drawLine(
  lineCtx: LineDrawCtx,
  ctx: CanvasRenderingContext2D,
  line: string,
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
  const maxWidth = Math.max(prevMaxWidth, lineMs.width);
  const prevMaxRight = lineCtx.maxRight ?? 0;
  const maxRight = Math.max(prevMaxRight, lineCtx.x + lineMs.width);
  const prevMaxHeight = lineCtx.maxHeight ?? 0;
  const newHeight =
    lineCtx.y +
    lineMs.fontBoundingBoxAscent +
    lineMs.fontBoundingBoxDescent +
    lineCtx.lineGap;
  const maxHeight = Math.max(prevMaxHeight, newHeight);

  return {
    ...lineCtx,
    y: newHeight,
    lastMetrics: lineMs,
    maxWidth,
    maxRight,
    maxHeight,
  };
}

type ColDrawCtx = {
  lineDrawCtx: LineDrawCtx;
  columnGap: number;
  y0?: number;
};

export type Cell = {
  content: string;
  empty?: boolean;
};

export type Col = Cell[];

export type Row = Cell[];

// Re-structure a table from row-based to col-based
function reStructureTable(rows: Row[]): Col[] {
  if (rows.length === 0) {
    return [];
  }
  const nCols = rows[0].length;
  const cols: Col[] = [];
  for (let c = 0; c < nCols; c++) {
    const col: Col = [];
    for (let r = 0; r < rows.length; r++) {
      if (rows[r].length >= c + 1) {
        col.push(rows[r][c]);
      } else {
        col.push({ content: "", empty: true });
      }
    }
    cols.push(col);
  }
  return cols;
}

function drawCol(
  colCtx: ColDrawCtx,
  ctx: CanvasRenderingContext2D,
  column: Col,
): ColDrawCtx {
  const newColCtx: ColDrawCtx = {
    ...colCtx,
    y0: colCtx.y0 ?? colCtx.lineDrawCtx.y,
  };
  newColCtx.lineDrawCtx = {
    ...newColCtx.lineDrawCtx,
    maxWidth: 0,
    y: newColCtx.y0!,
  };

  for (const cell of column) {
    newColCtx.lineDrawCtx = drawLine(newColCtx.lineDrawCtx, ctx, cell.content);
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

type DimensionCtx = {
  w: number;
  h: number;
  maxW: number;
  maxH: number;
};

function doPaint(
  ctx: CanvasRenderingContext2D,
  canvasEle: HTMLCanvasElement,
  dim: DimensionCtx,
  preamble: Row[],
  tabularData: Row[],
): DimensionCtx {
  const dpi = window.devicePixelRatio;
  const canvasW = Math.max(dim.w * dpi, dim.maxW);
  const canvasH = Math.max(dim.h * dpi, dim.maxH);

  canvasEle.setAttribute("width", String(canvasW));
  canvasEle.setAttribute("height", String(canvasH));

  ctx.fillStyle = "#262626";
  ctx.fillRect(0, 0, canvasW, canvasH);

  const deltaX = 10 * dpi;
  const deltaY = 10 * dpi;

  ctx.setTransform(1, 0, 0, 1, deltaX, deltaY);

  let lineCtx: LineDrawCtx = {
    fillStyle: "#fff",
    baseline: "top",
    font: `${16 * dpi}px sans-serif`,
    y: 0,
    lineGap: 6 * dpi,
    x: 0,
  };

  for (const row of preamble) {
    if (row.length > 0) {
      lineCtx = drawLine(lineCtx, ctx, row[0].content);
    }
  }

  let colCtx: ColDrawCtx = {
    lineDrawCtx: lineCtx,
    columnGap: 30 * dpi,
  };

  const cols = reStructureTable(tabularData);

  for (const col of cols) {
    colCtx = drawCol(colCtx, ctx, col);
  }

  colCtx = {
    ...colCtx,
    lineDrawCtx: carriageReturn(colCtx.lineDrawCtx),
  };

  const measuredMaxR = colCtx.lineDrawCtx.maxRight ?? 0;
  const measuredMaxH = colCtx.lineDrawCtx.maxHeight ?? 0;

  return {
    ...dim,
    maxW: measuredMaxR + 2 * deltaX,
    maxH: measuredMaxH + 2 * deltaY,
  } as DimensionCtx;
}

export function CanvasTable(props: {
  preamble: Row[];
  tabularData: Row[];
  canvasRef: RefObject<HTMLCanvasElement | null>;
}) {
  const { preamble, tabularData, canvasRef } = props;
  const dimRef = useRef<DimensionCtx>({ w: 0, h: 0, maxW: 0, maxH: 0 });
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const ele = boxRef.current;
    if (!ele) {
      return;
    }

    const canvasEle = ele.querySelector("canvas");
    if (!canvasEle) {
      return;
    }

    const ctx = canvasEle.getContext("2d");
    if (!ctx) {
      return;
    }

    dimRef.current = doPaint(
      ctx,
      canvasEle,
      dimRef.current!,
      preamble,
      tabularData,
    );

    const obs = new ResizeObserver((entries) => {
      for (const ent of entries) {
        if (ent.target === ele) {
          const cr = ent.contentRect;
          if (cr) {
            dimRef.current = { ...dimRef.current, w: cr.width, h: cr.height };
            dimRef.current = doPaint(
              ctx,
              canvasEle,
              dimRef.current,
              preamble,
              tabularData,
            );
          }
        }
      }
    });

    obs.observe(ele);

    return () => {
      obs.unobserve(ele);
    };
  }, [preamble, tabularData]);

  return (
    <Fragment>
      <Box
        ref={boxRef}
        sx={{
          maxHeight: "80vh",
          overflow: "auto",
        }}
      >
        <canvas
          ref={canvasRef}
          style={{ width: "100%", height: "auto" }}
        ></canvas>
      </Box>
    </Fragment>
  );
}
