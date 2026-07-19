"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  api,
  clearTokens,
  getAccessToken,
  getRefreshToken,
  storeTokens,
} from "./api";
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

  useEffect(() => {
    let cancelled = false;

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
          if (!cancelled) setUserState(me);
        } catch {
          clearTokens();
        } finally {
          if (!cancelled) setLoading(false);
        }
        return;
      }

      if (tg?.initData) {
        try {
          const result = await api<LoginResult>("/auth/telegram-miniapp", {
            method: "POST",
            body: { initData: tg.initData },
          });
          if (!cancelled) {
            storeTokens(result.tokens);
            setUserState(result.user);
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
  }, []);

  const loginWithTelegram = useCallback(
    async (fields: TelegramAuthFields) => {
      const result = await api<LoginResult>("/auth/telegram", {
        method: "POST",
        body: fields,
      });
      storeTokens(result.tokens);
      setUserState(result.user);
    },
    [],
  );

  const logout = useCallback(async () => {
    const refreshToken = getRefreshToken();
    if (refreshToken) {
      await api("/auth/logout", {
        method: "POST",
        body: { refreshToken },
      }).catch(() => undefined);
    }
    clearTokens();
    setUserState(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        loginWithTelegram,
        logout,
        setUser: setUserState,
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
