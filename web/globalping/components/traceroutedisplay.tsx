"use client";

import {
  Box,
  Typography,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  TableContainer,
} from "@mui/material";
import { Fragment } from "react";
import { TaskCloseIconButton } from "@/components/taskclose";
import { PlayPauseButton } from "./playpause";

export function TracerouteResultDisplay(props: {}) {
  return (
    <Fragment>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="h6">Task #{1}</Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <PlayPauseButton
            running={true}
            onToggle={(prev, nxt) => {
              if (prev) {
                // todo
              } else {
                // todo
              }
            }}
          />

          <TaskCloseIconButton
            taskId={"1"}
            onConfirmedClosed={() => {
              // todo
            }}
          />
        </Box>
      </Box>
      <TableContainer sx={{ maxWidth: "100%", overflowX: "auto" }}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Hop</TableCell>

              <TableCell>Peers</TableCell>

              <TableCell>RTT</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            <TableRow>
              <TableCell>1</TableCell>
              <TableCell></TableCell>
              <TableCell></TableCell>
            </TableRow>
            <TableRow>
              <TableCell>2</TableCell>
              <TableCell></TableCell>
              <TableCell></TableCell>
            </TableRow>
            <TableRow>
              <TableCell>3</TableCell>
              <TableCell></TableCell>
              <TableCell></TableCell>
            </TableRow>
            <TableRow>
              <TableCell>4</TableCell>
              <TableCell></TableCell>
              <TableCell></TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>
    </Fragment>
  );
}
