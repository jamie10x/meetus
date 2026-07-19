// Minimal typing for the subset of the Telegram Mini App SDK
// (window.Telegram.WebApp) this project uses. Full reference:
// https://core.telegram.org/bots/webapps
export type TelegramWebApp = {
  initData: string;
  ready: () => void;
  expand: () => void;
  colorScheme: "light" | "dark";
};

declare global {
  interface Window {
    Telegram?: {
      WebApp?: TelegramWebApp;
    };
  }
}

/** Returns the Mini App SDK object, or null outside a Telegram client. */
export function getTelegramWebApp(): TelegramWebApp | null {
  if (typeof window === "undefined") return null;
  return window.Telegram?.WebApp ?? null;
}

/** True when initData is present — i.e. we're actually running inside Telegram. */
export function isTelegramMiniApp(): boolean {
  return !!getTelegramWebApp()?.initData;
}
