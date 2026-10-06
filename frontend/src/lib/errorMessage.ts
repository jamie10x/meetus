import { ApiError } from "./api";

const details = {
  "ticket already checked in": "alreadyCheckedIn",
  "ticket is not for this event": "wrongEvent",
  "this RSVP is not active": "inactiveTicket",
  "event is not open for check-in": "checkInClosed",
  "capacity cannot change while attendees are waitlisted": "capacityWaitlist",
  "capacity cannot be below confirmed attendance": "capacityAttendance",
  "account is banned": "banned",
} as const;
const codes = {
  validation_error: "validation", unauthorized: "unauthorized", forbidden: "forbidden",
  not_found: "notFound", conflict: "conflict", rate_limited: "rateLimited",
} as const;
export type ErrorMessageKey = typeof details[keyof typeof details] | typeof codes[keyof typeof codes];

// Translate known API failures, keeping an operation-specific translated
// fallback for network/internal errors. Never display arbitrary exception text.
export function errorMessage(error: unknown, translate: (key: ErrorMessageKey) => string, fallback: string): string {
  if (!(error instanceof ApiError)) return fallback;
  const detail = Object.prototype.hasOwnProperty.call(details, error.message)
    ? details[error.message as keyof typeof details] : undefined;
  const code = Object.prototype.hasOwnProperty.call(codes, error.code)
    ? codes[error.code as keyof typeof codes] : undefined;
  return detail || code ? translate(detail ?? code!) : fallback;
}
