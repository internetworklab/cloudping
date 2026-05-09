"use client";

import { useMemo, useState } from "react";
import {
  Box,
  Typography,
  Card,
  Divider,
  IconButton,
  Tooltip,
  CircularProgress,
  Button,
} from "@mui/material";
import StorageIcon from "@mui/icons-material/Storage";
import RefreshIcon from "@mui/icons-material/Refresh";

import {
  MRTEntriesLister,
  MRTEntry,
  ResumableResponseStreamEvent,
  useMRTEntriesReadByProvider,
} from "@/apis/mrtProviders";
import {
  defaultRouteQueryType,
  getQueryTypeLabel,
  PendingTask,
} from "@/apis/types";
import { TaskCloseIconButton } from "@/components/taskclose";
import { firstLetterCap } from "./strings";
import { RouteQueryStatsData, RouteQueryStatsDisplay } from "./RouteQueryStats";
import { MRTEntryCard } from "./MRTEntryCard";
import { SourceTabs } from "./SourceTabs";

export function RouteQueryResultDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
  mrtEntriesLister: MRTEntriesLister;
}) {
  const { task, onDeleted, mrtEntriesLister } = props;

  const [generation, setGeneration] = useState(0);
  const [activeProvider, setActiveProvider] = useState<string>(
    task.sources[0] ?? "",
  );
  const [loadedPagesData, setLoadedPagesData] = useState<
    Record<string, ResumableResponseStreamEvent[][]>
  >({});

  const { providerMap, loadMore } = useMRTEntriesReadByProvider(
    mrtEntriesLister,
    task.sources,
    task.routeQueryType,
    task.routeQueryTgt,
    generation,
  );

  // Reset active provider if the task sources change
  const validProvider = task.sources.includes(activeProvider)
    ? activeProvider
    : (task.sources[0] ?? "");

  const activeState = providerMap[validProvider];
  const mrtEntries = useMemo(
    () => activeState?.entries ?? [],
    [activeState?.entries],
  );
  const isRunning = activeState?.isRunning ?? false;
  const error = activeState?.error;
  const cursorId = activeState?.cursorId;

  // Flatten loaded pages + current page into a single entries list.
  const allEntries = useMemo(() => {
    const loadedEntries = (loadedPagesData[validProvider] ?? [])
      .flat()
      .map((e) => e.data?.Data)
      .filter((e): e is MRTEntry => e != null);
    return [...loadedEntries, ...mrtEntries];
  }, [loadedPagesData, validProvider, mrtEntries]);

  // Aggregate "any running" across all providers (for the global spinner)
  const anyRunning = Object.values(providerMap).some((s) => s.isRunning);

  const stats: RouteQueryStatsData = useMemo(() => {
    const origins = new Set<number>();
    const peerASs = new Set<number>();
    let maxPathLen = 0;
    let minPathLen = Infinity;
    for (const e of allEntries) {
      if (e.ASPath && e.ASPath.length > 0) {
        origins.add(e.ASPath[e.ASPath.length - 1]);
        maxPathLen = Math.max(maxPathLen, e.ASPath.length);
        minPathLen = Math.min(minPathLen, e.ASPath.length);
      }
      if (e.PeerAS !== undefined) peerASs.add(e.PeerAS);
    }
    return {
      count: allEntries.length,
      origins: origins.size,
      peers: peerASs.size,
      maxPathLen,
      minPathLen,
    };
  }, [allEntries]);

  const queryLabel = getQueryTypeLabel(
    task.routeQueryType ?? defaultRouteQueryType,
  );

  return (
    <Card>
      {/* Header */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          overflow: "hidden",
          padding: 2,
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
          }}
        >
          <StorageIcon sx={{ color: "primary.main", opacity: 0.9 }} />
          <Typography variant="h6">
            {firstLetterCap(task.type)} Task #{task.taskId}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          {anyRunning && <CircularProgress size={20} />}
          <Tooltip title="Refresh">
            <IconButton
              onClick={() => {
                setLoadedPagesData({});
                setGeneration((gen) => gen + 1);
              }}
            >
              <RefreshIcon />
            </IconButton>
          </Tooltip>
          <TaskCloseIconButton
            taskId={task.taskId}
            onConfirmedClosed={() => {
              onDeleted();
            }}
          />
        </Box>
      </Box>

      <Divider />

      {/* Provider tabs */}
      <SourceTabs
        tabs={task.sources}
        active={validProvider}
        onChange={(v) => setActiveProvider(v)}
      />

      {/* Current query description */}
      <Box
        sx={{
          px: 2,
          py: 1.5,
          display: "flex",
          alignItems: "center",
          gap: 0.5,
          flexWrap: "wrap",
          backgroundColor: "action.hover",
        }}
      >
        <Typography
          variant="body2"
          sx={{ color: "text.secondary", fontWeight: 400 }}
        >
          Query from
        </Typography>
        <Typography
          variant="body2"
          sx={{
            fontWeight: 600,
            color: "primary.main",
          }}
        >
          {validProvider}
        </Typography>
        <Typography
          variant="body2"
          sx={{ color: "text.secondary", fontWeight: 400 }}
        >
          where
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 600, color: "text.primary" }}
        >
          {queryLabel}
        </Typography>
        {task.routeQueryTgt && (
          <Typography
            variant="body2"
            sx={{
              fontFamily: '"Roboto Mono", monospace',
              color: "text.secondary",
              backgroundColor: "background.paper",
              px: 1,
              py: 0.25,
              borderRadius: 1,
              fontWeight: 500,
            }}
          >
            {task.routeQueryTgt}
          </Typography>
        )}
      </Box>

      {/* Stats */}
      <RouteQueryStatsDisplay data={stats} />

      <Divider />

      {/* Entries list */}
      <Box sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1.5 }}>
        {error && (
          <Box sx={{ textAlign: "center", py: 2 }}>
            <Typography variant="body2" sx={{ color: "error.main" }}>
              Error: {error}
            </Typography>
          </Box>
        )}

        {!isRunning && allEntries.length === 0 && !error && (
          <Box sx={{ textAlign: "center", py: 4 }}>
            <Typography variant="body2" sx={{ color: "text.secondary" }}>
              No MRT entries found.
            </Typography>
          </Box>
        )}

        {allEntries.map((entry, idx) => (
          <MRTEntryCard
            key={`${entry.Prefix}:${idx}`}
            entry={entry}
            index={idx}
            defaultExpanded
          />
        ))}

        {cursorId && !isRunning && (
          <Box sx={{ textAlign: "center", py: 2 }}>
            <Button
              variant="outlined"
              onClick={() => {
                const currentPageEvents = activeState?.pageEvents ?? [];
                setLoadedPagesData((prev) => ({
                  ...prev,
                  [validProvider]: [
                    ...(prev[validProvider] ?? []),
                    currentPageEvents,
                  ],
                }));
                loadMore(validProvider);
              }}
            >
              Load More
            </Button>
          </Box>
        )}
      </Box>
    </Card>
  );
}
