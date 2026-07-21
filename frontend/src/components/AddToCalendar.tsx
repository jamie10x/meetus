"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { downloadIcs, icsFilename, googleCalendarUrl, type IcsEvent } from "@/lib/ics";

type Props = {
  event: Omit<IcsEvent, "webUrl">;
  /** Path (e.g. "/events/5") the event lives at, resolved to an absolute
   *  URL client-side — kept out of the initial render so the .ics/Google
   *  Calendar links never differ between server and client markup. */
  path: string;
  className?: string;
};

export default function AddToCalendar({ event, path, className = "" }: Props) {
  const t = useTranslations("calendar");
  const [webUrl, setWebUrl] = useState<string | undefined>(undefined);

  useEffect(() => {
    setWebUrl(`${window.location.origin}${path}`);
  }, [path]);

  const full: IcsEvent = { ...event, webUrl };

  return (
    <div className={`flex flex-wrap items-center gap-2.5 text-sm ${className}`}>
      <button
        type="button"
        onClick={() => downloadIcs(full, icsFilename(event.title))}
        className="btn btn-secondary btn-sm"
      >
        {t("downloadIcs")}
      </button>
      <a
        href={googleCalendarUrl(full)}
        target="_blank"
        rel="noopener noreferrer"
        className="btn btn-secondary btn-sm"
      >
        {t("googleCalendar")}
      </a>
    </div>
  );
}
