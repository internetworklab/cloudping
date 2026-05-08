import { Box, Typography } from "@mui/material";

export interface RouteQueryStatsData {
  count: number;
  origins: number;
  peers: number;
  maxPathLen: number;
  minPathLen: number;
}

export function RouteQueryStatsDisplay(props: { data: RouteQueryStatsData }) {
  const stats = props.data;
  return (
    <Box
      sx={{
        display: "flex",
        flexWrap: "wrap",
        gap: 2,
        px: 2,
        py: 1.5,
        backgroundColor: "action.hover",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="caption"
          sx={{ color: "text.secondary", fontWeight: 500 }}
        >
          Entries
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 700, fontFamily: '"Roboto Mono", monospace' }}
        >
          {stats.count}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="caption"
          sx={{ color: "text.secondary", fontWeight: 500 }}
        >
          Origin ASNs
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 700, fontFamily: '"Roboto Mono", monospace' }}
        >
          {stats.origins}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="caption"
          sx={{ color: "text.secondary", fontWeight: 500 }}
        >
          Peer ASNs
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 700, fontFamily: '"Roboto Mono", monospace' }}
        >
          {stats.peers}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="caption"
          sx={{ color: "text.secondary", fontWeight: 500 }}
        >
          Shortest Path
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 700, fontFamily: '"Roboto Mono", monospace' }}
        >
          {stats.maxPathLen} hops
        </Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="caption"
          sx={{ color: "text.secondary", fontWeight: 500 }}
        >
          Longest Path
        </Typography>
        <Typography
          variant="body2"
          sx={{ fontWeight: 700, fontFamily: '"Roboto Mono", monospace' }}
        >
          {stats.minPathLen} hops
        </Typography>
      </Box>
    </Box>
  );
}
