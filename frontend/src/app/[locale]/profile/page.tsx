"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import { api, uploadImage, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { metaName, type MetaItem, type User } from "@/lib/types";

const LANGUAGES = [
  { value: "uz", label: "O'zbekcha" },
  { value: "ru", label: "Русский" },
  { value: "en", label: "English" },
] as const;

export default function ProfilePage() {
  const t = useTranslations("profile");
  const locale = useLocale();
  const { user, loading, setUser } = useAuth();
  const router = useRouter();

  const [cities, setCities] = useState<MetaItem[]>([]);
  const [name, setName] = useState("");
  const [cityId, setCityId] = useState<string>("");
  const [district, setDistrict] = useState("");
  const [language, setLanguage] = useState<string>("uz");
  const [avatarUrl, setAvatarUrl] = useState<string | null>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
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
    setAvatarUrl(user.avatarUrl);
  }, [user]);

  useEffect(() => {
    api<MetaItem[]>("/meta/cities").then(setCities).catch(() => setCities([]));
  }, []);

  if (loading || !user) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
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
          avatarUrl: avatarUrl || null,
        },
      });
      setUser(updated);
      setMessage(t("saved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const handleAvatarChange = async (file: File | undefined) => {
    if (!file) return;
    setUploadingAvatar(true);
    setError(null);
    try {
      setAvatarUrl(await uploadImage(file));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("avatarUploadFailed"));
    } finally {
      setUploadingAvatar(false);
    }
  };

  const inputCls =
    "rounded-xl border border-line bg-ink-raised px-3.5 py-2.5 text-bone placeholder:text-dust-dim transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20";

  return (
    <main className="mx-auto max-w-lg px-5 py-12">
      <h1 className="mb-6 font-display text-2xl font-black text-bone">{t("title")}</h1>

      <form onSubmit={save} className="flex flex-col gap-4">
        <div className="flex items-center gap-4">
          {avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={avatarUrl}
              alt=""
              className="h-16 w-16 rounded-full border border-line object-cover"
            />
          ) : (
            <span className="flex h-16 w-16 items-center justify-center rounded-full bg-ink-raised text-xl font-semibold text-bone">
              {name[0]}
            </span>
          )}
          <label className="btn btn-secondary btn-sm cursor-pointer">
            {uploadingAvatar ? t("uploading") : t("changePhoto")}
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(e) => handleAvatarChange(e.target.files?.[0])}
              disabled={uploadingAvatar}
              className="hidden"
            />
          </label>
        </div>

        <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
          {t("name")}
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className={inputCls}
          />
        </label>

        <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
          {t("city")}
          <select
            value={cityId}
            onChange={(e) => setCityId(e.target.value)}
            className={inputCls}
          >
            <option value="">{t("cityNotSet")}</option>
            {cities.map((c) => (
              <option key={c.id} value={c.id}>
                {metaName(c, locale)}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
          {t("district")}
          <input
            value={district}
            onChange={(e) => setDistrict(e.target.value)}
            placeholder={t("districtPlaceholder")}
            className={inputCls}
          />
        </label>

        <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
          {t("language")}
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            className={inputCls}
          >
            {LANGUAGES.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </select>
        </label>

        <button type="submit" disabled={saving} className="btn btn-primary mt-2">
          {saving ? t("saving") : t("save")}
        </button>

        {message ? <p className="text-sm text-registan-strong">{message}</p> : null}
        {error ? <p className="text-sm text-pomegranate">{error}</p> : null}
      </form>
    </main>
  );
}
