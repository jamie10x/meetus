import type { CSSProperties } from "react";

/**
 * Cover treatment per category slug — CSS-only patterns (no images), kept
 * disciplined to the two brand accents (registan cobalt, atlas gold) so a
 * grid of mixed categories still reads as one coherent palette instead of
 * an arbitrary per-category rainbow.
 */
const CATEGORY_COVERS: Record<string, CSSProperties> = {
  tech: {
    background:
      "repeating-linear-gradient(120deg, #12234c 0 16px, #2f6feb 16px 32px)",
  },
  education: {
    background:
      "repeating-linear-gradient(90deg, rgba(242,178,59,0.16) 0 1px, transparent 1px 22px)," +
      "repeating-linear-gradient(0deg, rgba(242,178,59,0.16) 0 1px, transparent 1px 22px), #172440",
  },
  business: {
    background:
      "repeating-linear-gradient(60deg, #b8811f 0 16px, #c6922c 16px 32px)",
  },
  sports: {
    background:
      "radial-gradient(circle at 70% 30%, #f2b23b 0%, #b8811f 45%, #6b4712 100%)",
  },
  social: {
    background: "linear-gradient(115deg, #12234c 0 48%, #b8811f 52% 100%)",
  },
  arts: {
    background:
      "radial-gradient(circle at 30% 30%, rgba(238,242,251,0.18) 0 2px, transparent 3px) 0 0/26px 26px," +
      "radial-gradient(circle at 70% 70%, rgba(238,242,251,0.14) 0 2px, transparent 3px) 0 0/26px 26px," +
      "linear-gradient(160deg, #12234c, #0c1a38)",
  },
  music: {
    background:
      "repeating-radial-gradient(circle at 50% 120%, #2f6feb 0 3px, #12234c 3px 14px, #1d4a9e 14px 26px)",
  },
  gaming: {
    background:
      "repeating-conic-gradient(from 0deg, #2f6feb 0deg 90deg, #101a30 90deg 180deg) 0 0/28px 28px",
  },
  language: {
    background:
      "radial-gradient(circle at 22% 35%, #2f6feb 0 9px, transparent 10px) 0 0/44px 44px," +
      "radial-gradient(circle at 66% 65%, #f2b23b 0 9px, transparent 10px) 0 0/44px 44px, #172440",
  },
  outdoor: {
    background: "linear-gradient(200deg, #f2b23b 0%, #b8811f 35%, #12234c 100%)",
  },
};

const FALLBACK_COVER: CSSProperties = {
  background: "linear-gradient(160deg, #1d4a9e, #101a30)",
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
