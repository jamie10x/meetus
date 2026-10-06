import type { TokenPair } from "./types";
import { forgetOffline, offlineTickets, rememberTickets, SESSION_KEY } from "./offline";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const ACCESS_KEY = "meetus.accessToken";
const REFRESH_KEY = "meetus.refreshToken";
export class ApiError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message); }
}
export function getAccessToken(): string | null { return typeof window === "undefined" ? null : localStorage.getItem(ACCESS_KEY); }
export function getRefreshToken(): string | null { return typeof window === "undefined" ? null : localStorage.getItem(REFRESH_KEY); }
export function storeTokens(tokens: TokenPair, newSession = true) {
  if (newSession || !localStorage.getItem(SESSION_KEY)) {
    forgetOffline();
    localStorage.setItem(SESSION_KEY, crypto.randomUUID());
  }
  localStorage.setItem(ACCESS_KEY, tokens.accessToken);
  localStorage.setItem(REFRESH_KEY, tokens.refreshToken);
}
export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
  localStorage.removeItem(SESSION_KEY);
  forgetOffline();
  window.dispatchEvent(new Event("meetus:logout"));
}
let refreshInFlight: Promise<boolean> | null = null;
async function tryRefresh(rejectedAccess: string | null): Promise<boolean> {
  if (!getRefreshToken()) return false;
  if (!refreshInFlight) {
    const session = localStorage.getItem(SESSION_KEY);
    const refresh = async () => {
      if (localStorage.getItem(SESSION_KEY) !== session) return false;
      // A sibling tab/request may have rotated while this request waited.
      if (getAccessToken() !== rejectedAccess) return !!getAccessToken();
      const refreshToken = getRefreshToken();
      if (!refreshToken) return false;
      const res = await fetch(`${API_URL}/api/auth/refresh`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken }), cache: "no-store",
      });
      if (!res.ok) {
        if (res.status === 401 || res.status === 403) {
          if (localStorage.getItem(SESSION_KEY) === session) clearTokens();
          throw new ApiError("unauthorized", "Session expired", res.status);
        }
        throw new ApiError("unavailable", "Session temporarily unavailable", res.status);
      }
      const body = await res.json();
      if (localStorage.getItem(SESSION_KEY) !== session || getRefreshToken() !== refreshToken) {
        // Logout/account switch won the race. Revoke the unused successor.
        void fetch(`${API_URL}/api/auth/logout`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ refreshToken: body.data.refreshToken }) }).catch(() => {});
        return false;
      }
      storeTokens(body.data as TokenPair, false);
      return true;
    };
    refreshInFlight = (async () => typeof navigator !== "undefined" && navigator.locks
      ? await navigator.locks.request("meetus:refresh", refresh) : await refresh())().finally(() => { refreshInFlight = null; });
  }
  return refreshInFlight!;
}
type RequestOptions = { method?: string; body?: unknown; auth?: boolean; signal?: AbortSignal };
async function request(path: string, init: RequestInit, auth: boolean): Promise<Response> {
  const session = typeof window === "undefined" ? null : localStorage.getItem(SESSION_KEY);
  let rejectedAccess: string | null = null;
  const send = () => {
    const headers = new Headers(init.headers);
    rejectedAccess = auth ? getAccessToken() : null;
    if (rejectedAccess) headers.set("Authorization", `Bearer ${rejectedAccess}`);
    return fetch(`${API_URL}/api${path}`, { ...init, headers, cache: "no-store" });
  };
  let res = await send();
  if (res.status === 401 && auth && await tryRefresh(rejectedAccess)) {
    if (session !== localStorage.getItem(SESSION_KEY)) throw new ApiError("unauthorized", "Session changed", 401);
    res = await send();
  }
  if (auth && session !== localStorage.getItem(SESSION_KEY)) throw new ApiError("unauthorized", "Session changed", 401);
  if (!res.ok) {
    const payload = await res.json().catch(() => null);
    throw new ApiError(payload?.error?.code ?? "internal_error", payload?.error?.message ?? "Request failed", res.status);
  }
  return res;
}
export async function api<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, auth = false, signal } = options;
  const session = typeof window === "undefined" ? null : localStorage.getItem(SESSION_KEY);
  try {
    const res = await request(path, { method, signal, headers: body === undefined ? undefined : { "Content-Type": "application/json" }, body: body === undefined ? undefined : JSON.stringify(body) }, auth);
    const payload = await res.json();
    if (auth && path === "/me/tickets" && method === "GET") rememberTickets(payload.data, session);
    return payload.data as T;
  } catch (error) {
    if (auth && path === "/me/tickets" && method === "GET" && error instanceof TypeError && session === localStorage.getItem(SESSION_KEY)) {
      const cached = offlineTickets<T>();
      if (cached !== undefined) return cached;
    }
    throw error;
  }
}
export async function downloadFile(path: string): Promise<Blob> { return (await request(path, {}, true)).blob(); }
export async function uploadImage(file: File): Promise<string> {
  const form = new FormData(); form.append("file", file);
  const res = await request("/uploads", { method: "POST", body: form }, true);
  return (await res.json()).data.url as string;
}
