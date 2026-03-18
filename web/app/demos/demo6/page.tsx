"use client";

import { Box, Paper, Typography } from "@mui/material";
import { useEffect, useRef, useState } from "react";

export interface EventObject {
  id: string;
  timestamp: number;
  message: string;
}

function EventDock(props: {
  eventsReader: ReadableStreamDefaultReader<EventObject>;
}) {
  const { eventsReader } = props;
  const [evs, setEVs] = useState<EventObject[]>([]);
  const tickRef = useRef<ReturnType<typeof setTimeout>>(undefined);

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
    <Box sx={{ padding: 1, display: "flex", flexDirection: "column", gap: 2 }}>
      {evs.map((ev) => (
        <Paper sx={{ padding: 1, borderRadius: 4 }} key={ev.id}>
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
    <Box>
      {readers.map((reader, i) => (
        <EventDock key={i} eventsReader={reader} />
      ))}
    </Box>
  );
}
