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
    if (!getAccessToken() && !getRefreshToken()) {
      setLoading(false);
      return;
    }
    api<User>("/me", { auth: true })
      .then(setUserState)
      .catch(() => clearTokens())
      .finally(() => setLoading(false));
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
