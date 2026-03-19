"use client";

import { EventObject } from "@/apis/types";
import { useEffect, useRef, useState } from "react";
import {
  FILTERKEY_CORR_ID,
  FILTERKEY_FROM,
  EventsBrowser,
} from "@/components/EventsBrowser";
import { Box } from "@mui/material";

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

const mockedSources: string[] = ["US-NYC1", "US-LAX1", "HK-HKG1", "SG-SIN1"];
const mockedDestinations: string[] = [
  "http://example.com",
  "https://bing.com",
  "https://x.com",
  "https://www.google.com/robots.txt",
];

const mockEVs: EventObject[] = [
  {
    id: "1",
    timestamp: Date.now(),
    message: "System started",
    labels: {
      [FILTERKEY_FROM]: "US-NYC1",
      [FILTERKEY_CORR_ID]: "https://www.google.com/robots.txt",
    },
  },
  {
    id: "2",
    timestamp: Date.now() + 1000,
    message: "Connection established",
    labels: {
      [FILTERKEY_FROM]: "US-LAX1",
      [FILTERKEY_CORR_ID]: "http://example.com",
    },
  },
  {
    id: "3",
    timestamp: Date.now() + 2000,
    message: "Data sync in progress",
    labels: {
      [FILTERKEY_FROM]: "HK-HKG1",
      [FILTERKEY_CORR_ID]: "https://bing.com",
    },
  },
  {
    id: "4",
    timestamp: Date.now() + 3000,
    message: "User logged in",
    labels: {
      [FILTERKEY_FROM]: "SG-SIN1",
      [FILTERKEY_CORR_ID]: "https://x.com",
    },
  },
  {
    id: "5",
    timestamp: Date.now() + 4000,
    message: "Cache cleared",
    labels: {
      [FILTERKEY_FROM]: "US-NYC1",
      [FILTERKEY_CORR_ID]: "https://www.google.com/robots.txt",
    },
  },
  {
    id: "6",
    timestamp: Date.now() + 5000,
    message: "Backup completed",
    labels: {
      [FILTERKEY_FROM]: "US-LAX1",
      [FILTERKEY_CORR_ID]: "http://example.com",
    },
  },
  {
    id: "7",
    timestamp: Date.now() + 6000,
    message: "System idle",
    labels: {
      [FILTERKEY_FROM]: "HK-HKG1",
      [FILTERKEY_CORR_ID]: "https://bing.com",
    },
  },
  {
    id: "8",
    timestamp: Date.now() + 7000,
    message: "Network request received",
    labels: {
      [FILTERKEY_FROM]: "SG-SIN1",
      [FILTERKEY_CORR_ID]: "https://x.com",
    },
  },
  {
    id: "9",
    timestamp: Date.now() + 8000,
    message: "Processing payload",
    labels: {
      [FILTERKEY_FROM]: "US-NYC1",
      [FILTERKEY_CORR_ID]: "https://www.google.com/robots.txt",
    },
  },
  {
    id: "10",
    timestamp: Date.now() + 9000,
    message: "Database query executed",
    labels: {
      [FILTERKEY_FROM]: "US-LAX1",
      [FILTERKEY_CORR_ID]: "http://example.com",
    },
  },
  {
    id: "11",
    timestamp: Date.now() + 10000,
    message: "Response sent",
    labels: {
      [FILTERKEY_FROM]: "HK-HKG1",
      [FILTERKEY_CORR_ID]: "https://bing.com",
    },
  },
  {
    id: "12",
    timestamp: Date.now() + 11000,
    message: "Memory optimized",
    labels: {
      [FILTERKEY_FROM]: "SG-SIN1",
      [FILTERKEY_CORR_ID]: "https://x.com",
    },
  },
  {
    id: "13",
    timestamp: Date.now() + 12000,
    message: "Session renewed",
    labels: {
      [FILTERKEY_FROM]: "US-NYC1",
      [FILTERKEY_CORR_ID]: "https://www.google.com/robots.txt",
    },
  },
  {
    id: "14",
    timestamp: Date.now() + 13000,
    message: "File uploaded",
    labels: {
      [FILTERKEY_FROM]: "US-LAX1",
      [FILTERKEY_CORR_ID]: "http://example.com",
    },
  },
  {
    id: "15",
    timestamp: Date.now() + 14000,
    message: "Email dispatched",
    labels: {
      [FILTERKEY_FROM]: "HK-HKG1",
      [FILTERKEY_CORR_ID]: "https://bing.com",
    },
  },
  {
    id: "16",
    timestamp: Date.now() + 15000,
    message: "Job queued",
    labels: {
      [FILTERKEY_FROM]: "SG-SIN1",
      [FILTERKEY_CORR_ID]: "https://x.com",
    },
  },
  {
    id: "17",
    timestamp: Date.now() + 16000,
    message: "Worker spawned",
    labels: {
      [FILTERKEY_FROM]: "US-NYC1",
      [FILTERKEY_CORR_ID]: "https://www.google.com/robots.txt",
    },
  },
  {
    id: "18",
    timestamp: Date.now() + 17000,
    message: "Task completed",
    labels: {
      [FILTERKEY_FROM]: "US-LAX1",
      [FILTERKEY_CORR_ID]: "http://example.com",
    },
  },
  {
    id: "19",
    timestamp: Date.now() + 18000,
    message: "Resources freed",
    labels: {
      [FILTERKEY_FROM]: "HK-HKG1",
      [FILTERKEY_CORR_ID]: "https://bing.com",
    },
  },
  {
    id: "20",
    timestamp: Date.now() + 19000,
    message: "System shutdown",
    labels: {
      [FILTERKEY_FROM]: "SG-SIN1",
      [FILTERKEY_CORR_ID]: "https://x.com",
    },
  },
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
    <Box
      sx={{
        height: "100vh",
        overflow: "hidden",
      }}
    >
      <EventsBrowser
        allSources={mockedSources}
        allDestinations={mockedDestinations}
        reader={readers && readers.length > 0 ? readers[0] : undefined}
      />
    </Box>
  );
}
