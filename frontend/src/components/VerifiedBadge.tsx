/**
 * Small inline checkmark shown next to a verified organizer's name.
 * Takes `label` as a prop rather than translating internally so it works
 * from both Client Components (`useTranslations`) and Server Components
 * (`getTranslations`) without caring which one the caller used.
 */
export default function VerifiedBadge({
  label,
  className = "",
}: {
  label: string;
  className?: string;
}) {
  return (
    <svg
      viewBox="0 0 20 20"
      aria-label={label}
      className={`inline-block h-3.5 w-3.5 shrink-0 align-[-2px] text-registan-strong ${className}`}
    >
      <title>{label}</title>
      <path
        fill="currentColor"
        d="M10 0l2.163 1.44 2.575-.394 1.19 2.33 2.33 1.19-.394 2.575L19.304 9.5l-1.44 2.163.394 2.575-2.33 1.19-1.19 2.33-2.575-.394L10 19.804l-2.163-1.44-2.575.394-1.19-2.33-2.33-1.19.394-2.575L.696 9.5l1.44-2.163-.394-2.575 2.33-1.19 1.19-2.33 2.575.394z"
      />
      <path
        fill="var(--color-ink)"
        d="M8.6 12.9L5.9 10.2l1.06-1.06 1.64 1.64 4.44-4.44 1.06 1.06z"
      />
    </svg>
  );
}
