"use client";

import { Fragment } from "react";

export function useSiteName() {
  const siteName = process.env["NEXT_PUBLIC_SITE_NAME"];
  return { siteName };
}

export function SiteName() {
  const { siteName } = useSiteName();
  return <Fragment>{siteName}</Fragment>;
}
