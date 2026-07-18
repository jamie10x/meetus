"use client";

import { useEffect, useState } from "react";
import { api, uploadImage, ApiError } from "@/lib/api";
import type { EventInput, EventItem, MetaItem } from "@/lib/types";

type Props = {
  initial?: EventItem;
  submitLabel: string;
  onSubmit: (input: EventInput) => Promise<void>;
};

/** Converts an RFC3339 timestamp to the value of a datetime-local input. */
function toLocalInput(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Converts a datetime-local value to RFC3339 in the browser's timezone. */
function toRFC3339(local: string): string {
  return new Date(local).toISOString();
}

const inputCls =
  "rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900";
const labelCls = "flex flex-col gap-1 text-sm font-medium";

export default function EventForm({ initial, submitLabel, onSubmit }: Props) {
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
  const [isOnline, setIsOnline] = useState(initial?.isOnline ?? false);
  const [startsAt, setStartsAt] = useState(
    initial ? toLocalInput(initial.startsAt) : "",
  );
  const [endsAt, setEndsAt] = useState(toLocalInput(initial?.endsAt ?? null));
  const [capacity, setCapacity] = useState(
    initial?.capacity ? String(initial.capacity) : "",
  );
  const [coverUrl, setCoverUrl] = useState(initial?.coverUrl ?? "");

  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api<MetaItem[]>("/meta/categories").then(setCategories).catch(() => {});
    api<MetaItem[]>("/meta/cities").then(setCities).catch(() => {});
  }, []);

  const handleCover = async (file: File | undefined) => {
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      setCoverUrl(await uploadImage(file));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Upload failed.");
    } finally {
      setUploading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await onSubmit({
        title,
        description,
        categoryId: Number(categoryId),
        cityId: cityId ? Number(cityId) : null,
        district: district || null,
        locationName: locationName || null,
        address: address || null,
        isOnline,
        startsAt: toRFC3339(startsAt),
        endsAt: endsAt ? toRFC3339(endsAt) : null,
        capacity: capacity ? Number(capacity) : null,
        coverUrl: coverUrl || null,
      });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save event.");
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <label className={labelCls}>
        Title
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
          maxLength={200}
          className={inputCls}
        />
      </label>

      <label className={labelCls}>
        Description
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={5}
          className={inputCls}
        />
      </label>

      <div className="grid grid-cols-2 gap-4">
        <label className={labelCls}>
          Category
          <select
            value={categoryId}
            onChange={(e) => setCategoryId(e.target.value)}
            required
            className={inputCls}
          >
            <option value="">Choose…</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nameEn}
              </option>
            ))}
          </select>
        </label>

        <label className={labelCls}>
          City
          <select
            value={cityId}
            onChange={(e) => setCityId(e.target.value)}
            className={inputCls}
          >
            <option value="">{isOnline ? "Not needed" : "Choose…"}</option>
            {cities.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nameEn}
              </option>
            ))}
          </select>
        </label>
      </div>

      <label className="flex items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          checked={isOnline}
          onChange={(e) => setIsOnline(e.target.checked)}
        />
        Online event
      </label>

      {!isOnline ? (
        <div className="grid grid-cols-2 gap-4">
          <label className={labelCls}>
            Venue name
            <input
              value={locationName}
              onChange={(e) => setLocationName(e.target.value)}
              placeholder="e.g. Impact Hub"
              className={inputCls}
            />
          </label>
          <label className={labelCls}>
            District
            <input
              value={district}
              onChange={(e) => setDistrict(e.target.value)}
              className={inputCls}
            />
          </label>
          <label className={`${labelCls} col-span-2`}>
            Address
            <input
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              className={inputCls}
            />
          </label>
        </div>
      ) : null}

      <div className="grid grid-cols-2 gap-4">
        <label className={labelCls}>
          Starts at
          <input
            type="datetime-local"
            value={startsAt}
            onChange={(e) => setStartsAt(e.target.value)}
            required
            className={inputCls}
          />
        </label>
        <label className={labelCls}>
          Ends at (optional)
          <input
            type="datetime-local"
            value={endsAt}
            onChange={(e) => setEndsAt(e.target.value)}
            className={inputCls}
          />
        </label>
      </div>

      <label className={labelCls}>
        Capacity (leave empty for unlimited)
        <input
          type="number"
          min={1}
          value={capacity}
          onChange={(e) => setCapacity(e.target.value)}
          className={inputCls}
        />
      </label>

      <label className={labelCls}>
        Cover image
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={(e) => handleCover(e.target.files?.[0])}
          className="text-sm"
        />
      </label>
      {uploading ? (
        <p className="text-sm text-zinc-500">Uploading…</p>
      ) : coverUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={coverUrl}
          alt="Cover preview"
          className="max-h-48 rounded-lg object-cover"
        />
      ) : null}

      <button
        type="submit"
        disabled={saving || uploading}
        className="mt-2 rounded-lg bg-sky-500 px-4 py-2 font-medium text-white hover:bg-sky-600 disabled:opacity-50"
      >
        {saving ? "Saving…" : submitLabel}
      </button>

      {error ? <p className="text-sm text-red-600">{error}</p> : null}
    </form>
  );
}
