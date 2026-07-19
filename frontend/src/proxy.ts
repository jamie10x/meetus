import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";

// Next.js 16 renamed the middleware convention to "proxy" — same
// NextRequest-based API, just a different file/export name. next-intl's
// middleware factory works unchanged; only this export wrapper is new.
export const proxy = createMiddleware(routing);

export const config = {
  // Skip API routes, static files, and anything with a file extension.
  matcher: ["/((?!api|_next|.*\\..*).*)"],
};
