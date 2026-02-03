"use client";

export function firstLetterCap(s: string): string {
  if (s && typeof s === "string" && s.length > 0) {
    return s[0].toUpperCase() + s.slice(1);
  }
  return s;
}
