"use client";

import { useEffect } from "react";

let navigationGuard: (() => boolean) | undefined;
export function hasUnsavedChanges(): boolean { return !!navigationGuard; }
export function confirmNavigation(): boolean { return navigationGuard?.() ?? true; }

/** Guard explicit navigation and browser unload without altering history. */
export function useUnsavedChanges(dirty: boolean, message: string) {
  useEffect(() => {
    if (!dirty) return;
    const guard = () => window.confirm(message);
    navigationGuard = guard;
    const unload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    const click = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const link = (event.target as Element).closest?.("a[href]") as HTMLAnchorElement | null;
      if (!link || link.hasAttribute("download") || (link.target && link.target !== "_self")) return;
      const next = new URL(link.href, window.location.href);
      if (next.pathname === window.location.pathname && next.search === window.location.search) return;
      if (!guard()) { event.preventDefault(); event.stopPropagation(); }
    };
    window.addEventListener("beforeunload", unload);
    document.addEventListener("click", click, true);
    return () => {
      if (navigationGuard === guard) navigationGuard = undefined;
      window.removeEventListener("beforeunload", unload);
      document.removeEventListener("click", click, true);
    };
  }, [dirty, message]);
}
