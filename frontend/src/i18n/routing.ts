import { defineRouting } from "next-intl/routing";

export const routing = defineRouting({
  locales: ["uz", "ru", "en"],
  // Majority-Uzbek audience, matches the backend's users.language default.
  defaultLocale: "uz",
  // Every locale gets a URL prefix (including the default) so each
  // language has its own clean, shareable, indexable URL — this matters
  // for this product specifically since event links are the viral loop
  // (see docs/architecture.md).
  localePrefix: "always",
});

export type AppLocale = (typeof routing.locales)[number];
