import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";

// Next.js 16 renamed the middleware convention to "proxy" — same
// NextRequest-based API, just a different file/export name. next-intl's
// middleware factory works unchanged; only this export wrapper is new.
export const proxy = createMiddleware(routing);

export const config = {
  // Skip API routes, static files, anything with a file extension, and
  // Next's generated icon routes — /icon and /apple-icon are served at
  // those exact extensionless paths (content-type comes from the
  // response header, not a URL suffix), so the ".*\\..*" file-extension
  // exclusion below doesn't catch them and they'd otherwise get
  // locale-redirected like a normal page.
  matcher: ["/((?!api|_next|icon|apple-icon|pwa-icon|.*\\..*).*)"],
};
