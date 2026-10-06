import type { User } from "./types";

const KEY = "meetus.offline.v2";
export const SESSION_KEY = "meetus.session";
type Snapshot = { session: string; user: User; expires: number; tickets?: unknown };
function read(): Snapshot | null {
  try {
    const value = JSON.parse(localStorage.getItem(KEY) ?? "null") as Snapshot | null;
    return value && value.session === localStorage.getItem(SESSION_KEY) && value.expires > Date.now() ? value : null;
  } catch { return null; }
}
function write(value: Snapshot) {
  try { localStorage.setItem(KEY, JSON.stringify(value)); } catch { /* Storage may be full or unavailable. */ }
}
export function forgetOffline() { localStorage.removeItem(KEY); }
export function rememberUser(user: User) {
  const session = localStorage.getItem(SESSION_KEY);
  if (!session) return;
  const previous = read();
  write({ session, user, expires: Date.now() + 30 * 86400000, tickets: previous?.user.id === user.id ? previous.tickets : undefined });
}
export function offlineUser(): User | null {
  const user = read()?.user;
  // An offline snapshot is for displaying tickets, never an authority grant.
  return user ? { ...user, isAdmin: false } : null;
}
export function rememberTickets(tickets: unknown, session: string | null) {
  const snapshot = read();
  if (snapshot && session === snapshot.session) write({ ...snapshot, tickets });
}
export function offlineTickets<T>(): T | undefined { return read()?.tickets as T | undefined; }
