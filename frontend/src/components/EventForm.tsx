"use client";

import { useUnsavedChanges } from "@/lib/useUnsavedChanges";
import { errorMessage } from "@/lib/errorMessage";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import dynamic from "next/dynamic";
import { api, uploadImage } from "@/lib/api";
import { categoryDotColor } from "@/lib/categoryStyle";
import { metaName, type EventInput, type EventItem, type MetaItem } from "@/lib/types";
import { toEventTimeInput as toLocalInput, fromEventTimeInput as toRFC3339 } from "@/lib/eventTime";
import DateTimeField from "@/components/DateTimeField";

// MapLibre touches `window` at import time — ssr: false keeps it out of
// the server bundle, same pattern as the Explore page's EventMap.
const LocationPicker = dynamic(() => import("@/components/LocationPicker"), {
  ssr: false,
  loading: () => (
    <div
      style={{ height: 260 }}
      className="w-full rounded-xl border border-line bg-ink-overlay"
    />
  ),
});

type Props = {
  initial?: EventItem;
  submitLabel: string;
  onSubmit: (input: EventInput) => Promise<void>;
};

const inputCls =
  "rounded-xl border border-line bg-ink-raised px-3.5 py-2.5 text-bone placeholder:text-dust-dim transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20";
const labelCls = "flex flex-col gap-1.5 text-sm font-medium text-dust";
const sectionTitleCls =
  "mt-2 border-t border-line pt-5 text-xs font-semibold uppercase tracking-wider text-dust-dim first:mt-0 first:border-t-0 first:pt-0";
const segBtnCls = (active: boolean) =>
  `rounded-full px-4 py-1.5 text-sm font-medium transition-all ${
    active
      ? "bg-gradient-to-br from-registan to-registan-dim text-[#f8fbff]"
      : "text-dust hover:text-bone"
  }`;

/** Small red asterisk after a label for fields that are actually required
 * — a visual hint only, the real enforcement is each input's `required`. */
function Req() {
  return (
    <span className="text-pomegranate" aria-hidden="true">
      {" "}
      *
    </span>
  );
}

export default function EventForm({ initial, submitLabel, onSubmit }: Props) {
  const t = useTranslations("eventForm");
  const tErrors = useTranslations("errors");
  const locale = useLocale();
  const [categories, setCategories] = useState<MetaItem[]>([]);
  const [cities, setCities] = useState<MetaItem[]>([]);

  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [categoryId, setCategoryId] = useState(
    initial ? String(initial.categoryId) : "",
  );
  const [cityId, setCityId] = useState(
    initial?.cityId ? String(initial.cityId) : "",
  );
  const [district, setDistrict] = useState(initial?.district ?? "");
  const [locationName, setLocationName] = useState(initial?.locationName ?? "");
  const [address, setAddress] = useState(initial?.address ?? "");
  const [lat, setLat] = useState(initial?.lat != null ? String(initial.lat) : "");
  const [lng, setLng] = useState(initial?.lng != null ? String(initial.lng) : "");
  const [isOnline, setIsOnline] = useState(initial?.isOnline ?? false);
  const [onlineUrl, setOnlineUrl] = useState(initial?.onlineUrl ?? "");
  const [startsAt, setStartsAt] = useState(
    initial ? toLocalInput(initial.startsAt) : "",
  );
  const [endsAt, setEndsAt] = useState(toLocalInput(initial?.endsAt ?? null));
  const [repeatsWeekly, setRepeatsWeekly] = useState(false);
  const [recurWeeks, setRecurWeeks] = useState("3");
  const [capacityMode, setCapacityMode] = useState<"unlimited" | "limited">(
    initial?.capacity ? "limited" : "unlimited",
  );
  const [capacity, setCapacity] = useState(
    initial?.capacity ? String(initial.capacity) : "",
  );
  const [coverUrl, setCoverUrl] = useState(initial?.coverUrl ?? "");
  const [coverPreview, setCoverPreview] = useState<string | null>(initial?.coverUrl ?? null);
  const [dragActive, setDragActive] = useState(false);

  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const values = JSON.stringify({ title, description, categoryId, cityId, district, locationName, address, lat, lng, isOnline, onlineUrl, startsAt, endsAt, repeatsWeekly, recurWeeks, capacityMode, capacity, coverUrl });
  const [originalValues] = useState(values);
  useUnsavedChanges(values !== originalValues || uploading, t("unsavedWarning"));

  useEffect(() => {
    api<MetaItem[]>("/meta/categories").then(setCategories).catch(() => {});
    api<MetaItem[]>("/meta/cities").then(setCities).catch(() => {});
  }, []);

  const handleCoverFile = async (file: File | undefined) => {
    if (!file) return;
    setCoverPreview(URL.createObjectURL(file));
    setUploading(true);
    setError(null);
    try {
      const url = await uploadImage(file);
      setCoverUrl(url);
      setCoverPreview(url);
    } catch (e) {
      setError(errorMessage(e, tErrors, t("uploadFailed")));
      setCoverPreview(coverUrl || null);
    } finally {
      setUploading(false);
    }
  };

  const endBeforeStart = Boolean(
    startsAt && endsAt && new Date(endsAt).getTime() <= new Date(startsAt).getTime(),
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!categoryId) {
      setError(t("categoryRequired"));
      return;
    }
    if (endBeforeStart) {
      setError(t("endBeforeStart"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await onSubmit({
        visibility: initial?.visibility ?? "public",
        title,
        description,
        categoryId: Number(categoryId),
        cityId: cityId ? Number(cityId) : null,
        district: district || null,
        locationName: locationName || null,
        address: address || null,
        lat: lat ? Number(lat) : null,
        lng: lng ? Number(lng) : null,
        isOnline,
        onlineUrl: isOnline && onlineUrl ? onlineUrl : null,
        startsAt: toRFC3339(startsAt),
        endsAt: endsAt ? toRFC3339(endsAt) : null,
        capacity: capacityMode === "limited" && capacity ? Number(capacity) : null,
        coverUrl: coverUrl || null,
        ...(!initial && repeatsWeekly ? { recurWeeks: Number(recurWeeks) } : {}),
      });
    } catch (err) {
      setError(errorMessage(err, tErrors, t("saveFailed")));
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <h2 className={sectionTitleCls}>{t("sectionBasics")}</h2>

      <label className={labelCls}>
        <span className="flex items-center justify-between">
          <span>
            {t("titleLabel")}
            <Req />
          </span>
          <span className="text-xs font-normal text-dust-dim">{title.length}/200</span>
        </span>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
          maxLength={200}
          className={inputCls}
        />
      </label>

      <label className={labelCls}>
        {t("description")}
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={5}
          className={inputCls}
        />
      </label>

      <div className={labelCls}>
        {t("category")}
        <Req />
        <div className="flex flex-wrap gap-2">
          {categories.map((c) => {
            const active = categoryId === String(c.id);
            return (
              <button
                key={c.id}
                type="button"
                aria-pressed={active}
                onClick={() => setCategoryId(String(c.id))}
                className={`flex items-center gap-1.5 rounded-full border px-3.5 py-2 text-sm font-medium transition-all ${
                  active
                    ? "border-registan bg-gradient-to-br from-registan to-registan-dim text-[#f8fbff] shadow-[0_6px_18px_-8px_rgba(47,111,235,0.7)]"
                    : "border-line bg-ink-raised text-dust hover:border-registan-dim hover:text-bone"
                }`}
              >
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ background: active ? "#f8fbff" : categoryDotColor(c.slug) }}
                />
                {metaName(c, locale)}
              </button>
            );
          })}
        </div>
      </div>

      <label className={labelCls}>
        {t("city")}
        <select
          value={cityId}
          onChange={(e) => setCityId(e.target.value)}
          className={inputCls}
        >
          <option value="">{isOnline ? t("notNeeded") : t("chooseOption")}</option>
          {cities.map((c) => (
            <option key={c.id} value={c.id}>
              {metaName(c, locale)}
            </option>
          ))}
        </select>
      </label>

      <h2 className={sectionTitleCls}>{t("sectionLocation")}</h2>

      <div className={labelCls}>
        {t("locationType")}
        <div className="inline-flex w-fit rounded-full border border-line bg-ink-raised p-1">
          <button type="button" onClick={() => setIsOnline(false)} className={segBtnCls(!isOnline)}>
            {t("locationTypeInPerson")}
          </button>
          <button type="button" onClick={() => setIsOnline(true)} className={segBtnCls(isOnline)}>
            {t("locationTypeOnline")}
          </button>
        </div>
      </div>

      {!isOnline ? (
        <div className="grid grid-cols-2 gap-4">
          <label className={labelCls}>
            {t("venueName")}
            <input
              value={locationName}
              onChange={(e) => setLocationName(e.target.value)}
              placeholder={t("venuePlaceholder")}
              className={inputCls}
            />
          </label>
          <label className={labelCls}>
            {t("district")}
            <input
              value={district}
              onChange={(e) => setDistrict(e.target.value)}
              className={inputCls}
            />
          </label>
          <label className={`${labelCls} col-span-2`}>
            {t("address")}
            <input
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              className={inputCls}
            />
          </label>
          <div className="col-span-2 flex flex-col gap-1.5">
            <span className="text-sm font-medium text-dust">{t("coordinates")}</span>
            <p className="text-xs text-dust-dim">{t("coordinatesHint")}</p>
            <LocationPicker
              lat={lat ? Number(lat) : null}
              lng={lng ? Number(lng) : null}
              onChange={(newLat, newLng) => {
                setLat(String(newLat));
                setLng(String(newLng));
              }}
              onAddressResolved={(resolved, source) =>
                // An explicit search pick wins; an address merely inferred
                // from dropping the pin only fills a blank field, so it
                // can't overwrite something typed by hand.
                setAddress((current) =>
                  source === "search" || !current ? resolved : current,
                )
              }
            />
            <div className="grid grid-cols-2 gap-4">
              <input
                type="number"
                step="any"
                min={-90}
                max={90}
                value={lat}
                onChange={(e) => setLat(e.target.value)}
                placeholder={t("latitude")}
                className={inputCls}
              />
              <input
                type="number"
                step="any"
                min={-180}
                max={180}
                value={lng}
                onChange={(e) => setLng(e.target.value)}
                placeholder={t("longitude")}
                className={inputCls}
              />
            </div>
          </div>
        </div>
      ) : (
        <label className={labelCls}>
          {t("meetingLink")}
          <input
            type="url"
            value={onlineUrl}
            onChange={(e) => setOnlineUrl(e.target.value)}
            placeholder={t("meetingLinkPlaceholder")}
            className={inputCls}
          />
          <span className="text-xs font-normal text-dust-dim">
            {t("meetingLinkHint")}
          </span>
        </label>
      )}

      <h2 className={sectionTitleCls}>{t("sectionSchedule")}</h2>
      <p className="text-sm text-dust">{t("timezone")}</p>

      <div className="flex flex-col gap-4 sm:flex-row">
        <label className={`${labelCls} flex-1`}>
          <span>
            {t("startsAt")}
            <Req />
          </span>
          <DateTimeField label={t("startsAt")} value={startsAt} onChange={setStartsAt} required />
        </label>
        <label className={`${labelCls} flex-1`}>
          {t("endsAtOptional")}
          <DateTimeField label={t("endsAtOptional")} value={endsAt} onChange={setEndsAt} min={startsAt} />
          {endBeforeStart ? (
            <span className="text-xs text-pomegranate">{t("endBeforeStart")}</span>
          ) : null}
        </label>
      </div>

      {!initial ? (
        <div className="flex flex-col gap-2.5 rounded-xl border border-line bg-ink-raised px-3.5 py-3">
          <label className="flex items-center gap-2 text-sm font-medium text-dust">
            <input
              type="checkbox"
              checked={repeatsWeekly}
              onChange={(e) => setRepeatsWeekly(e.target.checked)}
              className="h-4 w-4 rounded border-line bg-ink accent-registan"
            />
            {t("repeatsWeekly")}
          </label>
          {repeatsWeekly ? (
            <label className="flex flex-col gap-1.5 pl-6 text-sm font-medium text-dust">
              {t("recurWeeksLabel")}
              <select
                value={recurWeeks}
                onChange={(e) => setRecurWeeks(e.target.value)}
                className={inputCls}
              >
                {Array.from({ length: 11 }, (_, i) => i + 1).map((n) => (
                  <option key={n} value={n}>
                    {t("recurWeeksOption", { count: n + 1 })}
                  </option>
                ))}
              </select>
              <span className="text-xs font-normal text-dust-dim">
                {t("recurWeeksHint")}
              </span>
            </label>
          ) : null}
        </div>
      ) : null}

      <h2 className={sectionTitleCls}>{t("sectionCapacityMedia")}</h2>

      <div className={labelCls}>
        {t("capacityToggleLabel")}
        <div className="inline-flex w-fit rounded-full border border-line bg-ink-raised p-1">
          <button
            type="button"
            onClick={() => setCapacityMode("unlimited")}
            className={segBtnCls(capacityMode === "unlimited")}
          >
            {t("capacityUnlimited")}
          </button>
          <button
            type="button"
            onClick={() => setCapacityMode("limited")}
            className={segBtnCls(capacityMode === "limited")}
          >
            {t("capacityLimited")}
          </button>
        </div>
        {capacityMode === "limited" ? (
          <input
            type="number"
            min={1}
            required
            value={capacity}
            onChange={(e) => setCapacity(e.target.value)}
            className={`${inputCls} mt-1`}
          />
        ) : null}
      </div>

      <label className={labelCls}>
        {t("coverImage")}
        <div
          onDragOver={(e) => {
            e.preventDefault();
            setDragActive(true);
          }}
          onDragLeave={() => setDragActive(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragActive(false);
            handleCoverFile(e.dataTransfer.files?.[0]);
          }}
          className={`relative flex flex-col items-center justify-center gap-2.5 overflow-hidden rounded-xl border-2 border-dashed px-4 py-8 text-center transition-colors ${
            dragActive
              ? "border-registan-strong bg-registan/10"
              : "border-line bg-ink-raised"
          }`}
        >
          {coverPreview ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={coverPreview}
              alt={t("coverPreviewAlt")}
              className="max-h-40 rounded-lg object-cover"
            />
          ) : (
            <>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" aria-hidden="true" className="text-dust-dim">
                <path
                  d="M12 16V4M12 4l-4 4M12 4l4 4M4 16v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <p className="text-sm text-dust">{t("coverDropHint")}</p>
            </>
          )}
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={(e) => handleCoverFile(e.target.files?.[0])}
            className="absolute inset-0 z-0 cursor-pointer opacity-0"
          />
          {coverPreview ? (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setCoverPreview(null);
                setCoverUrl("");
              }}
              className="btn btn-outline btn-outline-danger btn-sm relative z-10"
            >
              {t("coverRemove")}
            </button>
          ) : null}
        </div>
      </label>
      {uploading ? <p className="text-sm text-dust">{t("uploading")}</p> : null}

      <details className="rounded-card border border-line bg-ink-raised p-5">
        <summary className="cursor-pointer font-semibold text-registan-strong">{t("preview")}</summary>
        <article className="mt-4 space-y-3">
          {coverPreview ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={coverPreview} alt="" className="h-40 w-full rounded-xl object-cover" />
          ) : null}
          <h3 className="break-words font-display text-2xl font-bold text-bone">{title || t("titleLabel")}</h3>
          <p className="font-mono text-sm text-registan-strong">{startsAt.replace("T", " · ")}</p>
          <p className="text-sm text-dust">{t("timezone")}</p>
          <p className="text-dust">{isOnline ? t("locationTypeOnline") : [locationName, address].filter(Boolean).join(" · ")}</p>
          <p className="whitespace-pre-wrap break-words text-dust">{description}</p>
        </article>
      </details>
      <button
        type="submit"
        disabled={saving || uploading}
        className="btn btn-primary mt-2"
      >
        {saving ? t("saving") : submitLabel}
      </button>

      {error ? <p role="alert" className="text-sm text-pomegranate">{error}</p> : null}
    </form>
  );
}
