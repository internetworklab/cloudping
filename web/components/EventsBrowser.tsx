"use client";

import { EventObject } from "@/apis/types";
import { useDockingMode } from "@/apis/useDockingMode";
import { Box, Chip, Paper, Typography } from "@mui/material";
import { useEffect, useRef, useState } from "react";

export const FILTERKEY_FROM = "from";
export const FILTERKEY_CORR_ID = "correlationId";

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
  eventsReader: ReadableStreamDefaultReader<EventObject>,
  labels: Record<string, string> | undefined,
) {
  const [allSrcs, setAllSrcs] = useState<string[]>([]);
  const [allDsts, setAllDsts] = useState<string[]>([]);
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
            const src = value.labels?.[FILTERKEY_FROM];

            if (src) {
              setAllSrcs((prev) => {
                const idx = prev.indexOf(src);
                return idx != -1 ? prev : [...prev, src].sort();
              });
            }
            const dst = value.labels?.[FILTERKEY_CORR_ID];
            if (dst) {
              setAllDsts((prev) =>
                prev.indexOf(dst) != -1 ? prev : [...prev, dst].sort(),
              );
            }
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
  return {
    evs: applyEVsLabelFilter(evs, labels).sort(
      (a, b) => a.timestamp - b.timestamp,
    ),
    allSrcs,
    allDsts,
  };
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

export function EventDock(props: { evs: EventObject[] }) {
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
          <Box sx={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
            {ev.message}
          </Box>
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
  reader: ReadableStreamDefaultReader<EventObject>;
}) {
  const { reader } = props;

  const [evLabelsFilter, setEVLabelsFilter] = useState<Record<string, string>>(
    {},
  );
  const currentActiveSource = evLabelsFilter[FILTERKEY_FROM];
  const currentActiveDest = evLabelsFilter[FILTERKEY_CORR_ID];

  const { evs, allDsts, allSrcs } = useEVsRead(reader, evLabelsFilter);

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
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
            {allSrcs.map((s, i) => (
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
            {allDsts.map((dest, i) => (
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
