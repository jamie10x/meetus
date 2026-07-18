"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { MetaItem, User } from "@/lib/types";

const LANGUAGES = [
  { value: "uz", label: "O'zbekcha" },
  { value: "ru", label: "Русский" },
  { value: "en", label: "English" },
] as const;

export default function ProfilePage() {
  const { user, loading, setUser } = useAuth();
  const router = useRouter();

  const [cities, setCities] = useState<MetaItem[]>([]);
  const [name, setName] = useState("");
  const [cityId, setCityId] = useState<string>("");
  const [district, setDistrict] = useState("");
  const [language, setLanguage] = useState<string>("uz");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);

  useEffect(() => {
    if (!user) return;
    setName(user.name);
    setCityId(user.cityId ? String(user.cityId) : "");
    setDistrict(user.district ?? "");
    setLanguage(user.language);
  }, [user]);

  useEffect(() => {
    api<MetaItem[]>("/meta/cities").then(setCities).catch(() => setCities([]));
  }, []);

  if (loading || !user) {
    return <main className="p-8 text-center text-zinc-500">Loading…</main>;
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage(null);
    setError(null);
    try {
      const updated = await api<User>("/me", {
        method: "PATCH",
        auth: true,
        body: {
          name,
          cityId: cityId ? Number(cityId) : null,
          district: district || null,
          language,
        },
      });
      setUser(updated);
      setMessage("Profile saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <main className="mx-auto max-w-lg px-4 py-10">
      <h1 className="mb-6 text-2xl font-bold">Your profile</h1>

      <form onSubmit={save} className="flex flex-col gap-4">
        <label className="flex flex-col gap-1 text-sm font-medium">
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium">
          City
          <select
            value={cityId}
            onChange={(e) => setCityId(e.target.value)}
            className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
          >
            <option value="">Not set</option>
            {cities.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nameEn}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium">
          District
          <input
            value={district}
            onChange={(e) => setDistrict(e.target.value)}
            placeholder="e.g. Chilonzor"
            className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium">
          Language
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
          >
            {LANGUAGES.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </select>
        </label>

        <button
          type="submit"
          disabled={saving}
          className="mt-2 rounded-lg bg-sky-500 px-4 py-2 font-medium text-white hover:bg-sky-600 disabled:opacity-50"
        >
          {saving ? "Saving…" : "Save"}
        </button>

        {message ? <p className="text-sm text-green-600">{message}</p> : null}
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
      </form>
    </main>
  );
}
