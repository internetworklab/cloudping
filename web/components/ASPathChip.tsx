import { Chip } from "@mui/material";

const ASN_COLORS = [
  "#e53935",
  "#1e88e5",
  "#43a047",
  "#fb8c00",
  "#8e24aa",
  "#00acc1",
  "#6d4c41",
  "#f06292",
  "#3949ab",
  "#7cb342",
  "#ff7043",
  "#5e35b1",
];

export function getASNColor(asn: number): string {
  return ASN_COLORS[Math.abs(asn) % ASN_COLORS.length];
}

export function ASPathChip(props: { asn: number }) {
  const { asn } = props;
  const color = getASNColor(asn);

  return (
    <Chip
      size="small"
      label={`AS${asn}`}
      sx={{
        backgroundColor: color + "18",
        color,
        border: `1px solid ${color}40`,
        fontWeight: 700,
        fontFamily: '"Roboto Mono", monospace',
        fontSize: "0.75rem",
        height: 24,
        borderRadius: "6px",
      }}
    />
  );
}
