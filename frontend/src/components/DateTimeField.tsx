"use client";

import { useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { DayPicker } from "react-day-picker";
import "react-day-picker/style.css";
import { enUS, ru, uz } from "react-day-picker/locale";

type Props = {
  label: string;
  value: string; // datetime-local format: "YYYY-MM-DDTHH:mm", or ""
  onChange: (value: string) => void;
  required?: boolean;
  min?: string;
};

function parseValue(value: string): { date: Date | undefined; time: string } {
  if (!value) return { date: undefined, time: "" };
  const [datePart, timePart] = value.split("T");
  const [y, m, d] = datePart.split("-").map(Number);
  return { date: new Date(y, m - 1, d), time: timePart ?? "" };
}

function toValue(date: Date, time: string): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  const datePart = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  return `${datePart}T${time || "00:00"}`;
}

/** Custom date + time picker replacing the native `datetime-local` input,
 * whose OS-chrome look clashes with the dark theme and can't be restyled.
 * Calendar grid comes from react-day-picker (themed via CSS vars in
 * globals.css); the time half stays a native `<input type="time">`, which
 * is already good UX (native wheel picker on mobile) and low-risk to
 * hand-roll a worse version of. */
export default function DateTimeField({ value, onChange, required, min, label }: Props) {
  const locale = useLocale();
  const t = useTranslations("dateTime");
  const { date, time } = parseValue(value);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const minDate = min ? parseValue(min).date : undefined;

  const dateLabel = date
    ? date.toLocaleDateString(locale, { weekday: "short", day: "numeric", month: "short", year: "numeric" })
    : null;

  return (
    <div ref={rootRef} className="relative flex gap-2">
      <button
        type="button"
        aria-label={t("date", { label })}
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex-1 rounded-xl border border-line bg-ink-raised px-3.5 py-2.5 text-left text-bone transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20"
      >
        {dateLabel ?? <span className="text-dust-dim">DD/MM/YYYY</span>}
      </button>
      <input
        type="time"
        aria-label={t("time", { label })}
        required={required}
        value={time}
        onChange={(e) => onChange(toValue(date ?? new Date(), e.target.value))}
        className="w-[6.75rem] shrink-0 rounded-xl border border-line bg-ink-raised px-2.5 py-2.5 text-bone transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20 [color-scheme:dark]"
      />
      {open ? (
        <div className="absolute left-0 top-[calc(100%+6px)] z-20 rounded-xl border border-line bg-ink-overlay p-2 shadow-pop">
          <DayPicker
            mode="single"
            required
            locale={locale === "ru" ? ru : locale === "uz" ? uz : enUS}
            selected={date}
            defaultMonth={date}
            disabled={minDate ? { before: minDate } : undefined}
            onSelect={(d) => {
              if (!d) return;
              onChange(toValue(d, time || "18:00"));
              setOpen(false);
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
