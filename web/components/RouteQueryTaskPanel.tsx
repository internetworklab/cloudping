import {
  defaultRouteQueryType,
  getQueryTypeLabel,
  getQueryTypeTextPlaceholder,
  PendingTask,
  RouteQueryType,
} from "@/apis/types";
import {
  Box,
  FormControl,
  FormLabel,
  RadioGroup,
  FormControlLabel,
  Radio,
  TextField,
} from "@mui/material";
import { Dispatch, SetStateAction, useState } from "react";

export function RouteQueryTaskPanel({
  pendingTask,
  setPendingTask,
}: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const queryTy = pendingTask.routeQueryType ?? defaultRouteQueryType;
  const setQueryTy = (ty: RouteQueryType) =>
    setPendingTask((prev) => ({ ...prev, routeQueryType: ty }));

  const tgt = pendingTask.routeQueryTgt ?? "";

  return (
    <Box>
      <Box sx={{ display: "flex", flexWrap: "wrap", columnGap: 2, rowGap: 1 }}>
        <FormControl>
          <FormLabel>Query By</FormLabel>
          <RadioGroup
            value={queryTy}
            onChange={(_, val) => setQueryTy(val as RouteQueryType)}
            row
          >
            <FormControlLabel
              control={<Radio />}
              value={RouteQueryType.AS_Path_Segs}
              label={getQueryTypeLabel(RouteQueryType.AS_Path_Segs)}
            />
            <FormControlLabel
              control={<Radio />}
              value={RouteQueryType.Neighbor_ASN}
              label={getQueryTypeLabel(RouteQueryType.Neighbor_ASN)}
            />
            <FormControlLabel
              control={<Radio />}
              value={RouteQueryType.Origin_ASN}
              label={getQueryTypeLabel(RouteQueryType.Origin_ASN)}
            />
            <FormControlLabel
              control={<Radio />}
              value={RouteQueryType.IP_Or_CIDR}
              label={getQueryTypeLabel(RouteQueryType.IP_Or_CIDR)}
            />
          </RadioGroup>
        </FormControl>
      </Box>
      <TextField
        variant="standard"
        fullWidth
        placeholder={getQueryTypeTextPlaceholder(queryTy)}
        label={getQueryTypeLabel(queryTy)}
        value={tgt}
        onChange={(e) => {
          setPendingTask((prev) => ({
            ...prev,
            routeQueryTgt: e.target.value,
          }));
        }}
      />
    </Box>
  );
}
