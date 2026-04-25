"use client";

import { Box, Tooltip, IconButton, Link, Paper } from "@mui/material";
import GitHubIcon from "@mui/icons-material/GitHub";
import MoreHorizIcon from "@mui/icons-material/MoreHoriz";
import TelegramIcon from "@mui/icons-material/Telegram";
import { ModeSelector } from "@/components/ModeSelector";
import { About } from "./about";
import { Fragment, useState } from "react";
import { GiveUsAStar } from "./GiveUsAStar";

export function HeaderBar() {
  const repoAddr = process.env["NEXT_PUBLIC_GITHUB_REPO"];
  const repoOwner = process.env["NEXT_PUBLIC_REPO_OWNER"];
  const repoName = process.env["NEXT_PUBLIC_REPO_NAME"];
  const tgInviteLink = process.env["NEXT_PUBLIC_TG_INVITE_LINK"];
  const [showAboutDialog, setShowAboutDialog] = useState<boolean>(false);

  return (
    <Fragment>
      <Paper
        sx={{
          position: "fixed",
          top: 0,
          left: 0,
          width: "100%",
          paddingLeft: 2,
          paddingRight: 2,
          paddingTop: 1,
          paddingBottom: 1,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 1,
          zIndex: 1,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
          {repoAddr !== "" && (
            <Tooltip title="Go to Project's Github Page">
              <Link
                underline="hover"
                href={repoAddr}
                target="_blank"
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
              >
                <GitHubIcon />
                Project
              </Link>
            </Tooltip>
          )}
          {repoOwner && repoName && (
            <GiveUsAStar repoOwner={repoOwner} repoName={repoName} />
          )}
          {!!tgInviteLink && (
            <Tooltip title={"Join our Telegram group"}>
              <Link
                underline="hover"
                href={tgInviteLink}
                target={"_blank"}
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
              >
                <TelegramIcon />
                Chat
              </Link>
            </Tooltip>
          )}
          <Tooltip title="More">
            <IconButton size="small" onClick={() => setShowAboutDialog(true)}>
              <MoreHorizIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            flexWrap: "wrap",
            gap: 1,
          }}
        >
          <ModeSelector />
        </Box>
      </Paper>
      <About open={showAboutDialog} onClose={() => setShowAboutDialog(false)} />
    </Fragment>
  );
}
