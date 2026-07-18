import type { TokenPair } from "./types";

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const ACCESS_KEY = "meetus.accessToken";
const REFRESH_KEY = "meetus.refreshToken";

export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_KEY);
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_KEY);
}

export function storeTokens(tokens: TokenPair) {
  localStorage.setItem(ACCESS_KEY, tokens.accessToken);
  localStorage.setItem(REFRESH_KEY, tokens.refreshToken);
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

// Single in-flight refresh shared by concurrent 401s.
let refreshInFlight: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      const refreshToken = getRefreshToken();
      if (!refreshToken) return false;
      try {
        const res = await fetch(`${API_URL}/api/auth/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refreshToken }),
        });
        if (!res.ok) return false;
        const body = await res.json();
        storeTokens(body.data as TokenPair);
        return true;
      } catch {
        return false;
      } finally {
        refreshInFlight = null;
      }
    })();
  }
  return refreshInFlight;
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  auth?: boolean;
};

/** Calls the API, unwraps the {data}/{error} envelope, auto-refreshes on 401. */
export async function api<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = "GET", body, auth = false } = options;

  const doFetch = async (): Promise<Response> => {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (auth) {
      const token = getAccessToken();
      if (token) headers["Authorization"] = `Bearer ${token}`;
    }
    return fetch(`${API_URL}/api${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  };

  let res = await doFetch();
  if (res.status === 401 && auth && (await tryRefresh())) {
    res = await doFetch();
  }

  const payload = await res.json().catch(() => null);
  if (!res.ok) {
    const err = payload?.error;
    throw new ApiError(
      err?.code ?? "internal_error",
      err?.message ?? "Request failed",
      res.status,
    );
  }
  return payload.data as T;
}

/** Uploads an image and returns its public URL. */
export async function uploadImage(file: File): Promise<string> {
  const doUpload = async (): Promise<Response> => {
    const form = new FormData();
    form.append("file", file);
    const headers: Record<string, string> = {};
    const token = getAccessToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
    return fetch(`${API_URL}/api/uploads`, {
      method: "POST",
      headers,
      body: form,
    });
  };

  let res = await doUpload();
  if (res.status === 401 && (await tryRefresh())) {
    res = await doUpload();
  }
  const payload = await res.json().catch(() => null);
  if (!res.ok) {
    const err = payload?.error;
    throw new ApiError(
      err?.code ?? "internal_error",
      err?.message ?? "Upload failed",
      res.status,
    );
  }
  return payload.data.url as string;
}
