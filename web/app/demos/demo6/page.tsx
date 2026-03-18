"use client";

import { Box, Paper, Typography } from "@mui/material";
import { RefObject, useEffect, useRef, useState } from "react";
import { div } from "three/src/nodes/TSL.js";

export interface EventObject {
  id: string;
  timestamp: number;
  message: string;
}

function useDockingMode(
  followingMode: RefObject<boolean>,
  containerRef: RefObject<HTMLDivElement | null>,
) {
  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "childList" && mutation.addedNodes.length > 0) {
          if (followingMode.current) {
            console.log("[dbg] scrolled:", container);
            container.scrollTop = Math.max(
              0,
              container.scrollHeight - container.clientHeight,
            );
          }
        }
      }
    });

    observer.observe(container, { childList: true });

    return () => {
      observer.disconnect();
    };
  }, []);
}

function EventDock(props: {
  eventsReader: ReadableStreamDefaultReader<EventObject>;
}) {
  const { eventsReader } = props;
  const [evs, setEVs] = useState<EventObject[]>([]);
  const tickRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const divRef = useRef<HTMLDivElement>(null);
  const followingMode = useRef<boolean>(true);
  useDockingMode(followingMode, divRef);

  useEffect(() => {
    const startTick = () => {
      tickRef.current = setTimeout(() =>
        eventsReader.read().then(({ value, done }) => {
          if (done || !tickRef.current) {
            return;
          }
          if (value) {
            setEVs((prev) => [...prev, value]);
          }
          startTick();
        }),
      );
    };

    startTick();

    return () => {
      const tick = tickRef.current;
      tickRef.current = undefined;
      if (tick) {
        clearTimeout(tick);
      }
    };
  }, [eventsReader]);

  return (
    <Box
      ref={divRef}
      sx={{
        height: "100%",
        overflow: "auto",
        padding: 1,
        display: "flex",
        flexDirection: "column",
        gap: 1,
      }}
      onScroll={(ev) => {
        if (ev.target) {
          const div = ev.target as HTMLDivElement;
          const burringZoneHeight = 10;
          const shouldEnableFollowingMode =
            Math.abs(div.scrollHeight - div.clientHeight) < burringZoneHeight ||
            Math.abs(div.scrollTop - (div.scrollHeight - div.clientHeight)) <
              burringZoneHeight;
          followingMode.current = shouldEnableFollowingMode;
        }
      }}
    >
      {evs.map((ev) => (
        <Paper
          sx={{
            paddingLeft: 2,
            paddingRight: 2,
            paddingTop: 1,
            paddingBottom: 1,
            borderRadius: 4,
          }}
          key={ev.id}
        >
          <Typography variant="caption">{`${new Date(ev.timestamp).toISOString()}`}</Typography>
          <Box>{ev.message}</Box>
        </Paper>
      ))}
    </Box>
  );
}

function arrayToStream(
  evs: EventObject[],
): ReadableStreamDefaultReader<EventObject> {
  let index = 0;
  let ticker: ReturnType<typeof setInterval>;
  const stream = new ReadableStream<EventObject>({
    start(controller) {
      console.log("[dbg] ev ticker started");
      ticker = setInterval(() => {
        console.log("[dbg] ev ticker ticked");
        if (index < evs.length) {
          controller.enqueue(evs[index]);
          index++;
        } else {
          clearInterval(ticker);
          controller.close();
        }
      }, 1000);
    },
    cancel() {
      if (ticker) {
        console.log("[dbg] ev ticker cancelled");
        clearInterval(ticker);
      }
    },
  });
  return stream.getReader();
}

const mockEVs: EventObject[] = [
  { id: "1", timestamp: Date.now(), message: "System started" },
  { id: "2", timestamp: Date.now() + 1000, message: "Connection established" },
  { id: "3", timestamp: Date.now() + 2000, message: "Data sync in progress" },
  { id: "4", timestamp: Date.now() + 3000, message: "User logged in" },
  { id: "5", timestamp: Date.now() + 4000, message: "Cache cleared" },
  { id: "6", timestamp: Date.now() + 5000, message: "Backup completed" },
  { id: "7", timestamp: Date.now() + 6000, message: "System idle" },
  {
    id: "8",
    timestamp: Date.now() + 7000,
    message: "Network request received",
  },
  { id: "9", timestamp: Date.now() + 8000, message: "Processing payload" },
  {
    id: "10",
    timestamp: Date.now() + 9000,
    message: "Database query executed",
  },
  { id: "11", timestamp: Date.now() + 10000, message: "Response sent" },
  { id: "12", timestamp: Date.now() + 11000, message: "Memory optimized" },
  { id: "13", timestamp: Date.now() + 12000, message: "Session renewed" },
  { id: "14", timestamp: Date.now() + 13000, message: "File uploaded" },
  { id: "15", timestamp: Date.now() + 14000, message: "Email dispatched" },
  { id: "16", timestamp: Date.now() + 15000, message: "Job queued" },
  { id: "17", timestamp: Date.now() + 16000, message: "Worker spawned" },
  { id: "18", timestamp: Date.now() + 17000, message: "Task completed" },
  { id: "19", timestamp: Date.now() + 18000, message: "Resources freed" },
  { id: "20", timestamp: Date.now() + 19000, message: "System shutdown" },
];

export default function Page() {
  const [readers, setReaders] = useState<
    ReadableStreamDefaultReader<EventObject>[]
  >([]);
  const readerRef = useRef<ReadableStreamDefaultReader<EventObject>>(undefined);
  useEffect(() => {
    if (readerRef.current) {
      return;
    }

    const it = setTimeout(() => {
      const streamReader = arrayToStream(mockEVs);
      readerRef.current = streamReader;
      setReaders([streamReader]);
    });

    return () => {
      clearTimeout(it);
      setReaders([]);
      if (readerRef.current) {
        const reader = readerRef.current;
        readerRef.current = undefined;
        reader
          .cancel()
          .catch((e) => console.error("failed to cancel stream:", e));
      }
    };
  }, []);
  return (
    <Box sx={{ height: "100vh", overflow: "hidden" }}>
      {readers.map((reader, i) => (
        <EventDock key={i} eventsReader={reader} />
      ))}
    </Box>
  );
}
