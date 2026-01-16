"use client";

import { getNodes } from "@/apis/globalping";
import {
  Dialog,
  DialogContent,
  Typography,
  DialogTitle,
  DialogActions,
  Button,
  Box,
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { Fragment } from "react";

export function About(props: { open: boolean; onClose: () => void }) {
  const { open, onClose } = props;
  const { data: webVersion } = useQuery({
    queryKey: ["version", "web"],
    queryFn: () =>
      fetch(
        process.env?.["NEXT_PUBLIC_WEB_VERSION_TXT"] || "/version.txt"
      ).then((res) => res.text()),
  });

  const { data: hubVersion } = useQuery({
    queryKey: ["version", "hub"],
    queryFn: () =>
      fetch(
        (process.env?.["NEXT_PUBLIC_API_ENDPOINT"] || "") + "/version"
      ).then((res) => res.json()),
  });

  const { data: conns } = useQuery({
    queryKey: ["nodes"],
    queryFn: () => getNodes(),
  });
  let nodeVersions: string[][] = [];
  if (conns) {
    const repeatSet = new Set<string>();
    for (const key in conns) {
      const entry = conns[key];
      const nodeName = entry.node_name;
      if (!nodeName) {
        continue;
      }
      if (repeatSet.has(nodeName)) {
        continue;
      }
      const nodeVersion: string[] = [nodeName];
      if (entry.attributes?.["Version"]) {
        try {
          const versionObj = JSON.parse(entry.attributes?.["Version"]);
          if (typeof versionObj === "object" && versionObj !== null) {
            nodeVersion.push(JSON.stringify(versionObj, null, 2));
            nodeVersions.push(nodeVersion);
            repeatSet.add(nodeName);
          }
        } catch (e) {
          console.error(e);
        }
      }
    }
  }

  return (
    <Dialog maxWidth="sm" fullWidth open={open} onClose={onClose}>
      <DialogTitle>About MyGlobalping</DialogTitle>
      <DialogContent>
        <Box>
          <Typography gutterBottom>Web Version</Typography>
          <Box sx={{ fontFamily: "monospace", whiteSpace: "pre-wrap" }}>
            {webVersion}
          </Box>
        </Box>
        <Box sx={{ marginTop: 2 }}>
          <Typography gutterBottom>Hub Version</Typography>
          <Box sx={{ fontFamily: "monospace", whiteSpace: "pre-wrap" }}>
            {JSON.stringify(hubVersion ?? {}, null, 2)}
          </Box>
        </Box>
        {nodeVersions.map((nodeVersion) => (
          <Box key={nodeVersion[0]} sx={{ marginTop: 2 }}>
            <Typography gutterBottom>
              Version of Node {nodeVersion[0]}
            </Typography>
            <Box sx={{ fontFamily: "monospace", whiteSpace: "pre-wrap" }}>
              {nodeVersion[1]}
            </Box>
          </Box>
        ))}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Good</Button>
      </DialogActions>
    </Dialog>
  );
}
