"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import EventForm from "@/components/EventForm";
import { api } from "@/lib/api";
import type { EventInput, EventItem } from "@/lib/types";

export default function NewEventPage() {
  const t = useTranslations("eventNew");
  const router = useRouter();

  const create = async (input: EventInput) => {
    await api<EventItem>("/events", {
      method: "POST",
      auth: true,
      body: input,
    });
    router.push("/organizer");
  };

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <h1 className="mb-6 text-2xl font-bold">{t("title")}</h1>
      <p className="mb-6 text-sm text-zinc-500">{t("subtitle")}</p>
      <EventForm submitLabel={t("createDraft")} onSubmit={create} />
    </main>
  );
}
