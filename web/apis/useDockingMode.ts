"use client";

import { RefObject, useEffect } from "react";

// this hook controles whether a scollable container should automatically
// scroll to the bottom when a child append event happens
export function useDockingMode(
  followingMode: RefObject<boolean>,
  containerRef: RefObject<HTMLDivElement | null>,
) {
  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "childList" && mutation.addedNodes.length > 0) {
          if (followingMode.current) {
            console.log("[dbg] scrolled:", container);
            container.scrollTop = Math.max(
              0,
              container.scrollHeight - container.clientHeight,
            );
          }
        }
      }
    });

    observer.observe(container, { childList: true });

    return () => {
      observer.disconnect();
    };
  }, []);
}
