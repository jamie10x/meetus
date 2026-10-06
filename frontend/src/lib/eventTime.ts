/** Event schedules use Uzbekistan time consistently in forms, cards and bot. */
export function toEventTimeInput(iso: string | null): string {
  if (!iso) return "";
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: "Asia/Tashkent", year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const part = (type: string) => parts.find(value => value.type === type)?.value;
  return `${part("year")}-${part("month")}-${part("day")}T${part("hour")}:${part("minute")}`;
}
export function fromEventTimeInput(local: string): string {
  return new Date(`${local}:00+05:00`).toISOString();
}
