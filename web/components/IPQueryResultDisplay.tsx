"use client";

import { Fragment, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Card,
  Divider,
  IconButton,
  Tooltip,
  CircularProgress,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  TableContainer,
} from "@mui/material";
import StorageIcon from "@mui/icons-material/Storage";
import RefreshIcon from "@mui/icons-material/Refresh";
import ScreenRotationIcon from "@mui/icons-material/ScreenRotation";

import {
  IPQueryLister,
  IPQueryResultEntry,
  useIPQueryByProvider,
} from "@/apis/ip-query";
import { PendingTask } from "@/apis/types";
import { TaskCloseIconButton } from "@/components/taskclose";
import { firstLetterCap } from "./strings";
import { SourceTabs } from "./SourceTabs";

enum Orientation {
  IP_COL = "ip-col",
  ATTR_COL = "attr-col",
}

const ATTRS: {
  key: keyof NonNullable<IPQueryResultEntry["result"]>;
  label: string;
}[] = [
  { key: "ASN", label: "ASN" },
  { key: "ISP", label: "ISP" },
  { key: "Country", label: "Country" },
  { key: "Region", label: "Region" },
  { key: "City", label: "City" },
  { key: "Location", label: "Location" },
  { key: "Exact", label: "Coordinates" },
];

function formatValue(
  val: string | { Latitude: number; Longitude: number } | undefined | null,
): string | undefined {
  if (val === undefined || val === null) return undefined;
  if (typeof val === "object") {
    return `${val.Latitude.toFixed(4)}, ${val.Longitude.toFixed(4)}`;
  }
  return val;
}

function findEntry(
  results: IPQueryResultEntry[],
  ip: string,
): IPQueryResultEntry | undefined {
  return results.find((r) => r.ip === ip);
}

export function IPQueryResultDisplay(props: {
  task: PendingTask;
  onDeleted: () => void;
  ipQueryLister: IPQueryLister;
}) {
  const { task, onDeleted, ipQueryLister } = props;

  const [generation, setGeneration] = useState(0);
  const [activeProvider, setActiveProvider] = useState<string>(
    task.sources[0] ?? "",
  );
  const [orientation, setOrientation] = useState<Orientation>(
    Orientation.IP_COL,
  );

  const providerMap = useIPQueryByProvider(
    ipQueryLister,
    task.targets,
    task.sources.length > 0 ? task.sources : undefined,
    generation,
  );

  const validProvider = task.sources.includes(activeProvider)
    ? activeProvider
    : (task.sources[0] ?? "");

  const activeState = providerMap[validProvider];
  const results: IPQueryResultEntry[] = useMemo(
    () => activeState?.results ?? [],
    [activeState?.results],
  );
  const isRunning = activeState?.isRunning ?? false;
  const error = activeState?.error;

  const anyRunning = Object.values(providerMap).some((s) => s.isRunning);

  const uniqueASNs = useMemo(() => {
    const set = new Set<string>();
    for (const r of results) {
      if (r.result?.ASN) set.add(r.result.ASN);
    }
    return set.size;
  }, [results]);

  const uniqueCountries = useMemo(() => {
    const set = new Set<string>();
    for (const r of results) {
      if (r.result?.Country) set.add(r.result.Country);
    }
    return set.size;
  }, [results]);

  const uniqueISPs = useMemo(() => {
    const set = new Set<string>();
    for (const r of results) {
      if (r.result?.ISP) set.add(r.result.ISP);
    }
    return set.size;
  }, [results]);

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
          <Tooltip title="Rotate Orientation">
            <IconButton
              onClick={() =>
                setOrientation((prev) =>
                  prev === Orientation.IP_COL
                    ? Orientation.ATTR_COL
                    : Orientation.IP_COL,
                )
              }
            >
              <ScreenRotationIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title="Refresh">
            <IconButton
              onClick={() => {
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
        {task.targets.length > 0 && (
          <>
            <Typography
              variant="body2"
              sx={{ color: "text.secondary", fontWeight: 400 }}
            >
              for
            </Typography>
            {task.targets.map((ip) => (
              <Typography
                key={ip}
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
                {ip}
              </Typography>
            ))}
          </>
        )}
      </Box>

      {/* Stats bar */}
      <Box
        sx={{
          px: 2,
          py: 1,
          display: "flex",
          gap: 3,
          flexWrap: "wrap",
          borderBottom: "1px solid",
          borderColor: "divider",
        }}
      >
        <Box>
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            Results
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {results.length}
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            Unique ASNs
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {uniqueASNs}
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            Unique Countries
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {uniqueCountries}
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            Unique ISPs
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {uniqueISPs}
          </Typography>
        </Box>
      </Box>

      <Divider />

      {/* Table */}
      <TableContainer sx={{ maxWidth: "100%", overflowX: "auto" }}>
        <Table size="small">
          {orientation === Orientation.IP_COL ? (
            // Default: one column per IP, one row per attribute
            <Fragment>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 700 }}>Attribute</TableCell>
                  {task.targets.map((ip, idx) => (
                    <TableCell
                      key={`${ip}:${idx}`}
                      sx={{
                        fontWeight: 700,
                        fontFamily: '"Roboto Mono", monospace',
                      }}
                    >
                      {ip}
                    </TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {ATTRS.map((attr) => (
                  <TableRow key={attr.key}>
                    <TableCell sx={{ fontWeight: 600 }}>{attr.label}</TableCell>
                    {task.targets.map((ip, idx) => {
                      const e = findEntry(results, ip);
                      const val = formatValue(e?.result?.[attr.key]);
                      return (
                        <TableCell key={`${ip}:${idx}`}>
                          {e?.err ? (
                            <Typography
                              variant="body2"
                              sx={{ color: "error.main" }}
                            >
                              {e.err}
                            </Typography>
                          ) : isRunning && !e ? (
                            <CircularProgress size={14} />
                          ) : (
                            <Typography variant="body2">
                              {val ?? "—"}
                            </Typography>
                          )}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))}
                {error && (
                  <TableRow>
                    <TableCell
                      colSpan={task.targets.length + 1}
                      sx={{ color: "error.main" }}
                    >
                      Error: {error}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Fragment>
          ) : (
            // Alternative: one row per IP, one column per attribute
            <Fragment>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 700 }}>IP</TableCell>
                  {ATTRS.map((attr) => (
                    <TableCell key={attr.key} sx={{ fontWeight: 700 }}>
                      {attr.label}
                    </TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {task.targets.map((ip, idx) => {
                  const e = findEntry(results, ip);
                  return (
                    <TableRow key={`${ip}:${idx}`}>
                      <TableCell
                        sx={{
                          fontWeight: 600,
                          fontFamily: '"Roboto Mono", monospace',
                        }}
                      >
                        {ip}
                      </TableCell>
                      {ATTRS.map((attr) => {
                        const val = formatValue(e?.result?.[attr.key]);
                        return (
                          <TableCell key={attr.key}>
                            {e?.err ? (
                              <Typography
                                variant="body2"
                                sx={{ color: "error.main" }}
                              >
                                {e.err}
                              </Typography>
                            ) : isRunning && !e ? (
                              <CircularProgress size={14} />
                            ) : (
                              <Typography variant="body2">
                                {val ?? "—"}
                              </Typography>
                            )}
                          </TableCell>
                        );
                      })}
                    </TableRow>
                  );
                })}
                {error && (
                  <TableRow>
                    <TableCell
                      colSpan={ATTRS.length + 1}
                      sx={{ color: "error.main" }}
                    >
                      Error: {error}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Fragment>
          )}
        </Table>
      </TableContainer>

      {!isRunning && results.length === 0 && !error && (
        <Box sx={{ textAlign: "center", py: 4 }}>
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            No results found.
          </Typography>
        </Box>
      )}
    </Card>
  );
}
