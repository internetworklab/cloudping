"use client";

import { EventObject, HTTPProbeEvent, RawPingEvent } from "@/apis/types";
import { useEffect, useRef, useState } from "react";
import {
  FILTERKEY_CORR_ID,
  FILTERKEY_FROM,
  EventsBrowser,
} from "@/components/EventsBrowser";
import { Box } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { JSONLineDecoder, LineTokenizer } from "@/apis/globalping";

function convertRawPingEventToEventObj(
  rawPingEv: RawPingEvent<HTTPProbeEvent> | undefined | null,
  idx: number,
): EventObject | undefined {
  const from = rawPingEv?.metadata?.from;
  const corrId = rawPingEv?.data?.correlationId;
  const date = rawPingEv?.data?.transport?.Date;
  const name = rawPingEv?.data?.transport?.Name;
  const ty = rawPingEv?.data?.transport?.Type;
  const val = rawPingEv?.data?.transport?.Value;
  const err = rawPingEv?.data?.error;
  if (!from || !corrId || !date) {
    return;
  }
  const labels = {
    [FILTERKEY_FROM]: from,
    [FILTERKEY_CORR_ID]: corrId,
  };

  let tx: number;
  try {
    const x = Date.parse(date);
    if (Number.isNaN(x) || !Number.isFinite(x)) {
      console.error("failed to parse date:", x, "from", rawPingEv);
      return undefined;
    }
    tx = x;
  } catch (e) {
    console.error("failed to parse date:", e, "from", rawPingEv);
    return undefined;
  }

  if (err) {
    return {
      id: `err-${idx}`,
      message: String(err),
      timestamp: tx || Date.now(),
      labels,
    };
  }

  const evObj: EventObject = {
    id: `${from}:${corrId}:${idx}`,
    labels: labels,
    timestamp: tx,
    annotations: ty
      ? {
          Type: ty,
        }
      : undefined,
    message: `${name ?? ""}: ${val ?? ""}`,
  };
  console.log("[dbg] generated ev:", evObj);

  return evObj;
}

class EventAdapter extends TransformStream<unknown, EventObject> {
  constructor(private idx: number = 0) {
    super({
      transform(
        chunk: unknown,
        controller: TransformStreamDefaultController<unknown>,
      ) {
        const rawEventObj = chunk as RawPingEvent<HTTPProbeEvent>;
        if (!rawEventObj) {
          console.error("skipping nil raw ping event:", rawEventObj);
          return;
        }
        const evObj = convertRawPingEventToEventObj(rawEventObj, idx++);
        if (evObj) {
          controller.enqueue(evObj);
        } else {
          console.error("Ignore raw ping event:", rawEventObj);
        }
      },
    });
  }
}

export default function Page() {
  const { data: reader, isLoading } = useQuery({
    queryKey: ["/example_http_probe_1.json"],
    queryFn: () =>
      fetch("/example_http_probe_1.json")
        .then((r) => r.body)
        .then((r) => {
          return r
            ?.pipeThrough(new TextDecoderStream())
            .pipeThrough(new LineTokenizer())
            .pipeThrough(new JSONLineDecoder())
            .pipeThrough(new EventAdapter())
            .getReader();
        }),
  });

  return (
    <Box
      sx={{
        height: "100vh",
        overflow: "hidden",
      }}
    >
      {isLoading ? (
        <Box>Loading</Box>
      ) : reader ? (
        <EventsBrowser reader={reader} />
      ) : (
        <Box>No Data</Box>
      )}
    </Box>
  );
}
