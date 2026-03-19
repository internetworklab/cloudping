"use client";

import { EventObject } from "@/apis/types";
import { useDockingMode } from "@/apis/useDockingMode";
import { Box, Chip, Paper, Typography } from "@mui/material";
import { useEffect, useRef, useState } from "react";

function applyEVsLabelFilter(
  evs: EventObject[],
  labels: Record<string, string> | undefined,
): EventObject[] {
  if (!labels) {
    return evs;
  }
  return evs.filter((ev) => {
    for (const k in labels) {
      if (labels[k] != ev.labels?.[k]) {
        return false;
      }
    }
    return true;
  });
}

function useEVsRead(
  eventsReader: ReadableStreamDefaultReader<EventObject> | undefined,
  labels: Record<string, string> | undefined,
) {
  const [evs, setEVs] = useState<EventObject[]>([]);
  const tickRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => {
    if (!eventsReader) {
      return;
    }

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
  return { evs: applyEVsLabelFilter(evs, labels) };
}

function RenderChipsRow(props: { chips: Record<string, string> }) {
  const chips = dropNilValues(props.chips);
  const chipEntries: { key: string; value: string }[] = Object.entries(chips)
    .map(([k, v]) => ({ key: k, value: v }))
    .sort((a, b) => a.key.localeCompare(b.key));
  return (
    <Box
      sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}
    >
      {chipEntries.map((ent, i) => (
        <Chip
          key={`${ent.key}:${i}`}
          size="small"
          label={
            <Typography variant="caption">
              {ent.key}: {ent.value}
            </Typography>
          }
        />
      ))}
    </Box>
  );
}

function EventDock(props: { evs: EventObject[] }) {
  const { evs } = props;

  const divRef = useRef<HTMLDivElement>(null);
  const followingMode = useRef<boolean>(true);
  useDockingMode(followingMode, divRef);

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
            display: "flex",
            flexDirection: "column",
            gap: 1,
          }}
          key={ev.id}
        >
          <Typography variant="caption">{`${new Date(ev.timestamp).toISOString()}`}</Typography>
          <Box>{ev.message}</Box>
          {ev.labels && (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                flexWrap: "wrap",
                gap: 1,
              }}
            >
              <Typography variant="caption">Labels:</Typography>
              <RenderChipsRow chips={ev.labels} />
            </Box>
          )}
          {ev.annotations && (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                flexWrap: "wrap",
                gap: 1,
              }}
            >
              <Typography variant="caption">Annotations:</Typography>
              <RenderChipsRow chips={ev.annotations} />
            </Box>
          )}
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

const FILTERKEY_FROM = "from";
const FILTERKEY_CORR_ID = "correlationId";

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

function dropNilValues(dict: Record<string, string>) {
  const newMap: Record<string, string> = {};
  for (const k in dict) {
    if (dict[k]) {
      newMap[k] = dict[k];
    }
  }
  return newMap;
}

export function EventsBrowser(props: {
  allSources: string[];
  allDestinations: string[];
}) {
  const { allSources, allDestinations } = props;
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

  const [evLabelsFilter, setEVLabelsFilter] = useState<Record<string, string>>(
    {},
  );
  const currentActiveSource = evLabelsFilter[FILTERKEY_FROM];
  const currentActiveDest = evLabelsFilter[FILTERKEY_CORR_ID];

  const { evs } = useEVsRead(
    readers && readers.length ? readers[0] : undefined,
    evLabelsFilter,
  );

  return (
    <Box
      sx={{
        height: "100vh",
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Box
        sx={{ padding: 1, display: "flex", flexDirection: "column", gap: 1 }}
      >
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: 1,
          }}
        >
          <Box>From:</Box>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "center",
              gap: 1,
            }}
          >
            {allSources.map((s, i) => (
              <Chip
                key={`${s}:${i}`}
                color={s === currentActiveSource ? "primary" : "default"}
                onClick={() =>
                  setEVLabelsFilter((prev) =>
                    dropNilValues({
                      ...prev,
                      [FILTERKEY_FROM]: s === currentActiveSource ? "" : s,
                    }),
                  )
                }
                label={s}
              />
            ))}
          </Box>
        </Box>
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: 1,
          }}
        >
          <Box>Destination:</Box>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "center",
              gap: 1,
            }}
          >
            {allDestinations.map((dest, i) => (
              <Chip
                key={`${dest}:${i}`}
                color={dest === currentActiveDest ? "primary" : "default"}
                onClick={() =>
                  setEVLabelsFilter((prev) =>
                    dropNilValues({
                      ...prev,
                      [FILTERKEY_CORR_ID]:
                        dest === currentActiveDest ? "" : dest,
                    }),
                  )
                }
                label={dest}
              />
            ))}
          </Box>
        </Box>
      </Box>
      <Box sx={{ flex: "1", overflow: "hidden" }}>
        <EventDock evs={evs} />
      </Box>
    </Box>
  );
}

export default function Page() {
  return (
    <EventsBrowser
      allSources={mockedSources}
      allDestinations={mockedDestinations}
    />
  );
}
