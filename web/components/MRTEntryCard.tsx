import { MRTEntry } from "@/apis/mrtProviders";
import {
  Box,
  Chip,
  Collapse,
  Divider,
  IconButton,
  Paper,
  Typography,
} from "@mui/material";
import { useState } from "react";
import { ASPathChip, getASNColor } from "./ASPathChip";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import RouteIcon from "@mui/icons-material/Route";
import HubIcon from "@mui/icons-material/Hub";

function formatCommunity(c: number): string {
  const high = (c >>> 16) & 0xffff;
  const low = c & 0xffff;
  return `${high}:${low}`;
}

function formatLargeCommunity(c: [number, number, number]): string {
  return `${c[0]}:${c[1]}:${c[2]}`;
}

function formatExtendedCommunity(c: number): string {
  return `0x${c.toString(16).toUpperCase().padStart(16, "0")}`;
}

export function MRTEntryCard(props: {
  entry: MRTEntry;
  index: number;
  defaultExpanded: boolean;
}) {
  const { entry, index, defaultExpanded } = props;
  const [expanded, setExpanded] = useState(defaultExpanded);

  const hasCommunities =
    (entry.Communities && entry.Communities.length > 0) ||
    (entry.LargeCommunities && entry.LargeCommunities.length > 0) ||
    (entry.ExtendedCommunities && entry.ExtendedCommunities.length > 0);

  return (
    <Paper
      elevation={0}
      sx={{
        border: "1px solid",
        borderColor: "divider",
        borderRadius: 2,
        overflow: "hidden",
        transition: "border-color 0.2s, box-shadow 0.2s",
        "&:hover": {
          borderColor: "primary.main",
          boxShadow: "0 2px 8px rgba(0,0,0,0.06)",
        },
      }}
    >
      {/* Header row */}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          px: 2,
          py: 1.5,
          cursor: "pointer",
          userSelect: "none",
        }}
        onClick={() => setExpanded((p) => !p)}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
          }}
        >
          <Chip
            size="small"
            label={`#${index + 1}`}
            sx={{
              backgroundColor: "action.selected",
              color: "text.secondary",
              fontFamily: '"Roboto Mono", monospace',
              fontSize: "0.7rem",
              height: 20,
            }}
          />
          <Typography
            variant="body1"
            sx={{
              fontFamily: '"Roboto Mono", monospace',
              fontWeight: 700,
              color: "text.primary",
              letterSpacing: "-0.02em",
            }}
          >
            {entry.Prefix}
          </Typography>
          {entry.PeerAS !== undefined && (
            <Chip
              size="small"
              label={`Peer AS${entry.PeerAS}`}
              sx={{
                backgroundColor: getASNColor(entry.PeerAS) + "15",
                color: getASNColor(entry.PeerAS),
                fontWeight: 600,
                fontFamily: '"Roboto Mono", monospace',
                fontSize: "0.7rem",
                height: 20,
                borderRadius: "4px",
              }}
            />
          )}
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {hasCommunities && !expanded && (
            <Chip
              size="small"
              label="Details"
              sx={{
                backgroundColor: "warning.light",
                color: "warning.contrastText",
                fontSize: "0.65rem",
                height: 18,
                display: { xs: "none", sm: "inline-flex" },
              }}
            />
          )}
          <IconButton size="small" sx={{ color: "text.secondary" }}>
            {expanded ? (
              <ExpandLessIcon fontSize="small" />
            ) : (
              <ExpandMoreIcon fontSize="small" />
            )}
          </IconButton>
        </Box>
      </Box>

      {/* AS path preview (always visible) */}
      {entry.ASPath && entry.ASPath.length > 0 && (
        <Box
          sx={{
            px: 2,
            pb: 1.5,
            display: "flex",
            flexWrap: "wrap",
            gap: 0.5,
            alignItems: "center",
          }}
        >
          <Typography
            variant="caption"
            sx={{ color: "text.secondary", mr: 1, fontWeight: 500 }}
          >
            AS Path:
          </Typography>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              gap: 1,
              alignItems: "center",
            }}
          >
            {entry.ASPath.map((asn, idx) => (
              <ASPathChip key={idx} asn={asn} />
            ))}
          </Box>
        </Box>
      )}

      {/* Expanded details */}
      <Collapse in={expanded}>
        <Divider />
        <Box
          sx={{
            px: 2,
            py: 1.5,
            display: "flex",
            flexDirection: "column",
            gap: 1.5,
          }}
        >
          {/* Peer & NextHop */}
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <HubIcon
                fontSize="small"
                sx={{ color: "text.secondary", opacity: 0.7 }}
              />
              <Typography variant="body2" sx={{ color: "text.secondary" }}>
                Peer:
              </Typography>
              <Typography
                variant="body2"
                sx={{ fontFamily: '"Roboto Mono", monospace' }}
              >
                {entry.Peer ?? "—"}
              </Typography>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <RouteIcon
                fontSize="small"
                sx={{ color: "text.secondary", opacity: 0.7 }}
              />
              <Typography variant="body2" sx={{ color: "text.secondary" }}>
                Next Hop:
              </Typography>
              <Typography
                variant="body2"
                sx={{ fontFamily: '"Roboto Mono", monospace' }}
              >
                {entry.NextHop ?? "—"}
              </Typography>
            </Box>
          </Box>

          {/* Communities */}
          {hasCommunities && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              {entry.Communities && entry.Communities.length > 0 && (
                <Box
                  sx={{
                    display: "flex",
                    flexWrap: "wrap",
                    gap: 0.75,
                    alignItems: "center",
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{ color: "text.secondary", fontWeight: 500, mr: 0.5 }}
                  >
                    BGP Community:
                  </Typography>
                  {entry.Communities.map((c, i) => (
                    <Chip
                      key={i}
                      size="small"
                      label={formatCommunity(c)}
                      sx={{
                        backgroundColor: "action.hover",
                        color: "text.secondary",
                        fontFamily: '"Roboto Mono", monospace',
                        fontSize: "0.7rem",
                        height: 20,
                        borderRadius: "4px",
                      }}
                    />
                  ))}
                </Box>
              )}
              {entry.LargeCommunities && entry.LargeCommunities.length > 0 && (
                <Box
                  sx={{
                    display: "flex",
                    flexWrap: "wrap",
                    gap: 0.75,
                    alignItems: "center",
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{ color: "text.secondary", fontWeight: 500, mr: 0.5 }}
                  >
                    BGP Large Community:
                  </Typography>
                  {entry.LargeCommunities.map((c, i) => (
                    <Chip
                      key={i}
                      size="small"
                      label={formatLargeCommunity(c)}
                      sx={{
                        backgroundColor: "info.light",
                        color: "info.contrastText",
                        fontFamily: '"Roboto Mono", monospace',
                        fontSize: "0.7rem",
                        height: 20,
                        borderRadius: "4px",
                      }}
                    />
                  ))}
                </Box>
              )}
              {entry.ExtendedCommunities &&
                entry.ExtendedCommunities.length > 0 && (
                  <Box
                    sx={{
                      display: "flex",
                      flexWrap: "wrap",
                      gap: 0.75,
                      alignItems: "center",
                    }}
                  >
                    <Typography
                      variant="caption"
                      sx={{ color: "text.secondary", fontWeight: 500, mr: 0.5 }}
                    >
                      BGP Extended Community:
                    </Typography>
                    {entry.ExtendedCommunities.map((c, i) => (
                      <Chip
                        key={i}
                        size="small"
                        label={formatExtendedCommunity(c)}
                        sx={{
                          backgroundColor: "success.light",
                          color: "success.contrastText",
                          fontFamily: '"Roboto Mono", monospace',
                          fontSize: "0.7rem",
                          height: 20,
                          borderRadius: "4px",
                        }}
                      />
                    ))}
                  </Box>
                )}
            </Box>
          )}
        </Box>
      </Collapse>
    </Paper>
  );
}
