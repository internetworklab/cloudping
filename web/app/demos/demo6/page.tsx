"use client";

import { Box } from "@mui/material";
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
    <Box>
      {evs.map((ev) => (
        <Box key={ev.id}>{ev.message}</Box>
      ))}
    </Box>
  );
}

function arrayToStream(
  evs: EventObject[],
): ReadableStreamDefaultReader<EventObject> {}

export default function Page() {
  return <Box></Box>;
}
