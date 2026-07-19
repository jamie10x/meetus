import { createNavigation } from "next-intl/navigation";
import { routing } from "./routing";

// Locale-aware Link/redirect/usePathname/useRouter — use these instead of
// next/link and next/navigation everywhere under src/app/[locale].
export const { Link, redirect, usePathname, useRouter, getPathname } =
  createNavigation(routing);
