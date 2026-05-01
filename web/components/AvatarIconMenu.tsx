"use client";

import { useState, useCallback } from "react";
import {
  IconButton,
  Menu,
  MenuItem,
  Avatar,
  Typography,
  Divider,
} from "@mui/material";
import PersonOutlineIcon from "@mui/icons-material/PersonOutline";
import { useBasicProfile } from "@/apis/basicprofile";

function stringToHue(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  return Math.abs(hash) % 360;
}

function getInitial(username: string): string {
  if (!username) {
    return "";
  }
  return username.charAt(0).toUpperCase();
}

export function AvatarIconMenu() {
  const { data: profile } = useBasicProfile();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleClick = useCallback((event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  }, []);

  const handleClose = useCallback(() => {
    setAnchorEl(null);
  }, []);

  const handleLogout = useCallback(async () => {
    handleClose();
    await fetch("/login/exit", { method: "POST" });
    window.location.reload();
  }, [handleClose]);

  const isLoggedIn = !!profile;
  const username = profile?.username ?? "";
  const initial = getInitial(username);
  const hue = stringToHue(username || "guest");
  const bgColor = `hsl(${hue}, 50%, 50%)`;

  return (
    <>
      <IconButton onClick={handleClick}>
        {isLoggedIn ? (
          <Avatar
            sx={{
              bgcolor: bgColor,
              color: "#fff",
              fontSize: 14,
              fontWeight: "bolder",
              width: 24,
              height: 24,
            }}
          >
            {initial}
          </Avatar>
        ) : (
          <PersonOutlineIcon />
        )}
      </IconButton>
      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        slotProps={{
          paper: {
            sx: {
              borderRadius: 4,
            },
          },
        }}
        anchorOrigin={{
          vertical: "bottom",
          horizontal: "right",
        }}
        transformOrigin={{
          vertical: "top",
          horizontal: "right",
        }}
      >
        {isLoggedIn && (
          <>
            <MenuItem disabled>
              <Typography variant="body2">Login as: {username}</Typography>
            </MenuItem>
            <Divider />
          </>
        )}
        {isLoggedIn ? (
          <MenuItem onClick={handleLogout}>
            <Typography variant="body2">Logout</Typography>
          </MenuItem>
        ) : (
          <MenuItem disabled>
            <Typography variant="body2">Not logged in</Typography>
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
