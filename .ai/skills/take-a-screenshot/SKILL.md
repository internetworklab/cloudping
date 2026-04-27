---
name: take-a-screenshot
description: Capture bright and dark mode screenshots of the local web dev server and combine them into a single comparison image. Use when the user asks for screenshots, visual captures, theme comparisons, or documentation images of the web frontend. Triggers on requests like "take a screenshot", "capture bright and dark mode", "screenshot the UI", or any visual snapshot task involving the web/ application.
---

# Take a Screenshot

Capture side-by-side bright and dark mode screenshots of the Next.js application running on the local dev server.

## Prerequisites

- Node.js and `npm`
- Chrome MCP server (stdio transport)
- `ffmpeg` for combining images

## Step 1: Start the Dev Server

Read `web/package.json` to confirm the dev port. This project uses `next dev -p 45844`, so the base URL is `http://localhost:45844`.

Check if the server is already responding:

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:45844
```

If the response is not `200`, start it:

```bash
cd web && npm run dev
```

Poll the port until it returns HTTP 200 before proceeding.

## Step 2: Capture Screenshots via Chrome MCP

Use the Chrome MCP server (stdio transport) to automate the browser.

### Preferred Method: Chrome Launch Flags

Force the color scheme at the browser level by passing Chrome flags when launching the instance. This affects `prefers-color-scheme` globally without requiring query parameters or page interaction.

**Bright mode:**
- Launch Chrome with `--force-light-mode`
- Navigate to `http://localhost:45844`
- Capture screenshot
- Save as `bright.png`

**Dark mode:**
- Launch Chrome with `--force-dark-mode`
- Navigate to `http://localhost:45844`
- Capture screenshot
- Save as `dark.png`

If the Chrome MCP server reuses an already-running Chrome instance and cannot relaunch with different flags, use one of the fallbacks below.

### Fallback 1: Query Parameters

Append theme-forcing query parameters to the URL:

- Bright: `http://localhost:45844?forceBright=true`
- Dark: `http://localhost:45844?forceDark=true`

### Fallback 2: JavaScript / CDP

If query parameters do not exist in the application, use Chrome DevTools Protocol or execute JavaScript in the page to toggle the theme. For example, inject a script that overrides `window.matchMedia` or forces MUI/Tailwind into the desired color scheme.

Save both images to a temporary directory (e.g., `/tmp/screenshots/`).

## Step 3: Combine Images

Combine the two screenshots into a single diagonal-split comparison image using `ffmpeg`:

```bash
ffmpeg -i /tmp/screenshots/dark.png -i /tmp/screenshots/bright.png -filter_complex \
"[0:v]format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='if(lte(X, W*0.65 - Y*(W*0.2/H)), 255, 0)'[left]; \
 [1:v][left]overlay=format=auto" \
/tmp/screenshots/combined.png
```

This overlays the dark screenshot diagonally onto the bright screenshot. Adjust the diagonal angle by modifying the `0.65` and `0.2` coefficients in the alpha expression.

For a simple horizontal split instead:

```bash
ffmpeg -i /tmp/screenshots/bright.png -i /tmp/screenshots/dark.png -filter_complex hstack /tmp/screenshots/combined.png
```

Move the final `combined.png` to the path requested by the user. If no path is specified, leave it in the working directory as `combined.png`.

## Output

- `bright.png` — Bright/light mode screenshot
- `dark.png` — Dark mode screenshot
- `combined.png` — Combined comparison image
