import type { CSSProperties } from "react";

/**
 * Cover treatment per category slug — CSS-only patterns (no images), kept
 * disciplined to the two brand accents (registan teal, atlas gold) so a
 * grid of mixed categories still reads as one coherent palette instead of
 * an arbitrary per-category rainbow.
 */
const CATEGORY_COVERS: Record<string, CSSProperties> = {
  tech: {
    background:
      "repeating-linear-gradient(120deg, #0D6F67 0 16px, #14958A 16px 32px)",
  },
  education: {
    background:
      "repeating-linear-gradient(90deg, rgba(242,167,59,0.16) 0 1px, transparent 1px 22px)," +
      "repeating-linear-gradient(0deg, rgba(242,167,59,0.16) 0 1px, transparent 1px 22px), #2C1F2C",
  },
  business: {
    background:
      "repeating-linear-gradient(60deg, #B87B22 0 16px, #C68A2A 16px 32px)",
  },
  sports: {
    background:
      "radial-gradient(circle at 70% 30%, #F2A73B 0%, #B87B22 45%, #6B4712 100%)",
  },
  social: {
    background: "linear-gradient(115deg, #0D6F67 0 48%, #B87B22 52% 100%)",
  },
  arts: {
    background:
      "radial-gradient(circle at 30% 30%, rgba(246,239,228,0.18) 0 2px, transparent 3px) 0 0/26px 26px," +
      "radial-gradient(circle at 70% 70%, rgba(246,239,228,0.14) 0 2px, transparent 3px) 0 0/26px 26px," +
      "linear-gradient(160deg, #0D6F67, #0B5951)",
  },
  music: {
    background:
      "repeating-radial-gradient(circle at 50% 120%, #18ADA0 0 3px, #0D6F67 3px 14px, #14958A 14px 26px)",
  },
  gaming: {
    background:
      "repeating-conic-gradient(from 0deg, #18ADA0 0deg 90deg, #211722 90deg 180deg) 0 0/28px 28px",
  },
  language: {
    background:
      "radial-gradient(circle at 22% 35%, #18ADA0 0 9px, transparent 10px) 0 0/44px 44px," +
      "radial-gradient(circle at 66% 65%, #F2A73B 0 9px, transparent 10px) 0 0/44px 44px, #241A26",
  },
  outdoor: {
    background: "linear-gradient(200deg, #F2A73B 0%, #B87B22 35%, #0D6F67 100%)",
  },
};

const FALLBACK_COVER: CSSProperties = {
  background: "linear-gradient(160deg, #0D6F67, #211722)",
};

export function categoryCoverStyle(slug: string): CSSProperties {
  return CATEGORY_COVERS[slug] ?? FALLBACK_COVER;
}

/** Text color for the category label chip, tuned per cover for contrast. */
const CATEGORY_LABEL_TONE: Record<string, "registan" | "atlas" | "bone"> = {
  tech: "registan",
  education: "atlas",
  business: "atlas",
  sports: "atlas",
  social: "bone",
  arts: "registan",
  music: "registan",
  gaming: "registan",
  language: "bone",
  outdoor: "atlas",
};

export function categoryLabelClass(slug: string): string {
  const tone = CATEGORY_LABEL_TONE[slug] ?? "bone";
  return tone === "registan"
    ? "text-registan-strong"
    : tone === "atlas"
      ? "text-atlas"
      : "text-bone";
}
