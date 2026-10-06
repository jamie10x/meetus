"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  useRef,
  type ReactNode,
} from "react";
import {
  api,
  ApiError,
  clearTokens,
  getAccessToken,
  getRefreshToken,
  storeTokens,
} from "./api";
import { rememberUser, offlineUser, SESSION_KEY } from "./offline";
import { getTelegramWebApp } from "./telegram-webapp";
import type { LoginResult, TelegramAuthFields, User } from "./types";

type AuthContextValue = {
  user: User | null;
  loading: boolean;
  loginWithTelegram: (fields: TelegramAuthFields) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUserState] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const epoch = useRef(0);

  const setUser = useCallback((value: User) => { rememberUser(value); setUserState(value); }, []);

  useEffect(() => {
    let cancelled = false;
    const attempt = epoch.current;
    if ((getAccessToken() || getRefreshToken()) && !localStorage.getItem(SESSION_KEY)) localStorage.setItem(SESSION_KEY, crypto.randomUUID());
    const initialSession = localStorage.getItem(SESSION_KEY);

    // The SDK loads with strategy="beforeInteractive" (see layout.tsx),
    // so window.Telegram.WebApp is already populated by the time this
    // mount effect runs — no polling needed.
    const tg = getTelegramWebApp();
    tg?.ready();
    tg?.expand();

    (async () => {
      if (getAccessToken() || getRefreshToken()) {
        // An existing session takes priority over Mini App auto-login.
        try {
          const me = await api<User>("/me", { auth: true });
          if (!cancelled && attempt === epoch.current) { setUser(me); setLoading(false); }
          return;
        } catch (error) {
          if (cancelled || attempt !== epoch.current) return;
          if (!(error instanceof ApiError) || (error.status !== 401 && error.status !== 403)) {
            setUserState(offlineUser());
            setLoading(false);
            return;
          }
          if (localStorage.getItem(SESSION_KEY) === initialSession) clearTokens();
          if (error instanceof ApiError && error.status === 403) { setLoading(false); return; }
        }
      }

      if (tg?.initData && attempt === epoch.current) {
        try {
          const result = await api<LoginResult>("/auth/telegram-miniapp", {
            method: "POST",
            body: { initData: tg.initData },
          });
          if (!cancelled && attempt === epoch.current) {
            storeTokens(result.tokens);
            setUser(result.user);
          }
        } catch {
          // Not fatal — falls through to the normal Login Widget.
        }
      }
      if (!cancelled) setLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, [setUser]);

  useEffect(() => {
    const sync = (event: Event) => {
      if (event instanceof StorageEvent && event.key !== SESSION_KEY && event.key !== null) return;
      const attempt = ++epoch.current;
      setUserState(offlineUser());
      setLoading(false);
      if (getAccessToken() || getRefreshToken()) {
        void api<User>("/me", { auth: true }).then((me) => {
          if (attempt === epoch.current) setUser(me);
        }).catch(() => undefined);
      }
    };
    window.addEventListener("storage", sync);
    window.addEventListener("meetus:logout", sync);
    return () => { window.removeEventListener("storage", sync); window.removeEventListener("meetus:logout", sync); };
  }, [setUser]);

  const loginWithTelegram = useCallback(
    async (fields: TelegramAuthFields) => {
      const attempt = ++epoch.current;
      const result = await api<LoginResult>("/auth/telegram", {
        method: "POST",
        body: fields,
      });
      if (attempt !== epoch.current) return;
      storeTokens(result.tokens);
      setUser(result.user);
    },
    [setUser],
  );

  const logout = useCallback(async () => {
    const refreshToken = getRefreshToken();
    clearTokens();
    setUserState(null);
    if (refreshToken) {
      await api("/auth/logout", {
        method: "POST",
        body: { refreshToken },
      }).catch(() => undefined);
    }
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        loginWithTelegram,
        logout,
        setUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
