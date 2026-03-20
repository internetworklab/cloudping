"use client";

import { useColorScheme, Tooltip, IconButton } from "@mui/material";
import { Fragment } from "react";
import LightModeIcon from "@mui/icons-material/LightMode";
import DarkModeIcon from "@mui/icons-material/DarkMode";
import ContrastIcon from "@mui/icons-material/Contrast";
import { firstLetterCap } from "@/utls/strings";

export enum DarkModePreference {
  PreferSystem = "system",
  PreferDark = "dark",
  PreferLight = "light",
}

export const modes: DarkModePreference[] = [
  DarkModePreference.PreferSystem,
  DarkModePreference.PreferDark,
  DarkModePreference.PreferLight,
];

export function ModeSelector() {
  const { mode, setMode } = useColorScheme();
  if (!mode) {
    return null;
  }
  return (
    <Fragment>
      <Tooltip title={`Mode: ${firstLetterCap(mode)}`}>
        <IconButton
          onClick={() => {
            const idx = modes.findIndex((m) => m === mode);
            if (idx !== -1 && modes.length >= 1) {
              const nextIdx = (idx + 1) % modes.length;
              const nextMode = modes[nextIdx];
              setMode(nextMode as any);
            }
          }}
        >
          {mode === "light" ? (
            <LightModeIcon />
          ) : mode === "dark" ? (
            <DarkModeIcon />
          ) : mode === "system" ? (
            <ContrastIcon />
          ) : (
            <ContrastIcon />
          )}
        </IconButton>
      </Tooltip>
    </Fragment>
  );
}
