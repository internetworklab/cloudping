"use client";

import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import {
  CssBaseline,
  createTheme,
  ThemeProvider,
  useColorScheme,
  Box,
  Tooltip,
  IconButton,
} from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Fragment } from "react";
import LightModeIcon from "@mui/icons-material/LightMode";
import DarkModeIcon from "@mui/icons-material/DarkMode";
import ContrastIcon from "@mui/icons-material/Contrast";
import { firstLetterCap } from "@/components/strings";
import { useSiteName } from "@/components/sitename";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

const queryClient = new QueryClient();

const theme = createTheme({
  colorSchemes: {
    light: {
      palette: {
        background: {
          default: "#f5f5f5",
          paper: "#ffffff",
        },
      },
    },
    dark: true,
  },
  components: {
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: 16,
        },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: {
          borderRadius: 16,
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: 8,
        },
      },
    },
  },
});

const modes = ["system", "dark", "light"];

function ModeSelector() {
  const { mode, setMode } = useColorScheme();
  if (!mode) {
    return null;
  }
  return (
    <Fragment>
      <Box sx={{ position: "absolute", top: 0, right: 0 }}>
        <Box sx={{ padding: 1 }}>
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
        </Box>
      </Box>
    </Fragment>
  );
}

function WithMUITheme(props: { children: React.ReactNode }) {
  return (
    <ThemeProvider theme={theme}>
      <QueryClientProvider client={queryClient}>
        <CssBaseline />
        {props.children}
        <ModeSelector />
      </QueryClientProvider>
    </ThemeProvider>
  );
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const { siteName } = useSiteName();
  return (
    <html lang="en">
      <head>
        <title key="mytitle">{siteName}</title>
        <meta
          key="mydescription"
          name="description"
          content="Web-based Ping & Traceroute Tool"
        />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <WithMUITheme>{children}</WithMUITheme>
      </body>
    </html>
  );
}
