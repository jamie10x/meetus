"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { api, ApiError } from "@/lib/api";
import type { MetaItem } from "@/lib/types";

type Props = {
  resource: "cities" | "categories";
  heading: string;
};

const emptyForm = { slug: "", nameUz: "", nameRu: "", nameEn: "" };

/** Shared CRUD UI for the two admin-managed reference tables — cities and
 * categories differ only in endpoint and heading, so one component drives
 * both rather than duplicating the same list/edit/add interaction twice. */
export default function MetaManager({ resource, heading }: Props) {
  const t = useTranslations("admin");
  const [items, setItems] = useState<MetaItem[]>([]);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [error, setError] = useState<string | null>(null);

  const adminPath = `/admin/${resource}`;

  const load = useCallback(() => {
    api<MetaItem[]>(`/meta/${resource}`).then(setItems).catch(() => setItems([]));
  }, [resource]);
  useEffect(() => {
    load();
  }, [load]);

  const startEdit = (item: MetaItem) => {
    setAdding(false);
    setEditingId(item.id);
    setForm({
      slug: item.slug,
      nameUz: item.nameUz,
      nameRu: item.nameRu,
      nameEn: item.nameEn,
    });
  };

  const startAdd = () => {
    setEditingId(null);
    setAdding(true);
    setForm(emptyForm);
  };

  const cancel = () => {
    setEditingId(null);
    setAdding(false);
    setError(null);
  };

  const save = async () => {
    setError(null);
    try {
      if (adding) {
        await api(adminPath, { method: "POST", auth: true, body: form });
      } else if (editingId !== null) {
        await api(`${adminPath}/${editingId}`, {
          method: "PATCH",
          auth: true,
          body: form,
        });
      }
      cancel();
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("metaSaveFailed"));
    }
  };

  const remove = async (id: number) => {
    if (!window.confirm(t("metaDeleteConfirm"))) return;
    setError(null);
    try {
      await api(`${adminPath}/${id}`, { method: "DELETE", auth: true });
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("metaDeleteFailed"));
    }
  };

  const inputCls =
    "rounded-lg border border-line bg-ink-raised px-2 py-1 text-sm text-bone placeholder:text-dust-dim transition-colors focus:border-registan-dim";
  const btn =
    "rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors";

  const formRow = (
    <li className="flex flex-wrap items-center gap-2 p-3">
      <input
        value={form.slug}
        onChange={(e) => setForm({ ...form, slug: e.target.value })}
        placeholder={t("metaSlug")}
        className={`${inputCls} w-28`}
      />
      <input
        value={form.nameUz}
        onChange={(e) => setForm({ ...form, nameUz: e.target.value })}
        placeholder={t("metaNameUz")}
        className={`${inputCls} flex-1`}
      />
      <input
        value={form.nameRu}
        onChange={(e) => setForm({ ...form, nameRu: e.target.value })}
        placeholder={t("metaNameRu")}
        className={`${inputCls} flex-1`}
      />
      <input
        value={form.nameEn}
        onChange={(e) => setForm({ ...form, nameEn: e.target.value })}
        placeholder={t("metaNameEn")}
        className={`${inputCls} flex-1`}
      />
      <button
        onClick={save}
        className={`${btn} border-registan-dim text-registan-strong hover:bg-registan/[0.12]`}
      >
        {t("metaSave")}
      </button>
      <button
        onClick={cancel}
        className={`${btn} border-line text-dust hover:border-registan-strong hover:text-registan-strong`}
      >
        {t("metaCancel")}
      </button>
    </li>
  );

  return (
    <section className="mb-10">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-bone">{heading}</h2>
        <button
          onClick={startAdd}
          className={`${btn} border-registan-dim text-registan-strong hover:bg-registan/[0.12]`}
        >
          {t("metaAdd")}
        </button>
      </div>
      {error ? <p className="mb-2 text-sm text-pomegranate">{error}</p> : null}
      <ul className="divide-y divide-line rounded-card border border-line bg-ink-raised">
        {items.map((item) =>
          editingId === item.id ? (
            <div key={item.id}>{formRow}</div>
          ) : (
            <li key={item.id} className="flex items-center gap-3 p-3">
              <span className="w-28 shrink-0 font-mono text-xs text-dust-dim">
                {item.slug}
              </span>
              <span className="flex-1 truncate text-sm text-bone">
                {item.nameEn} · {item.nameRu} · {item.nameUz}
              </span>
              <button
                onClick={() => startEdit(item)}
                className={`${btn} border-line text-dust hover:border-registan-strong hover:text-registan-strong`}
              >
                {t("metaEdit")}
              </button>
              <button
                onClick={() => remove(item.id)}
                className={`${btn} border-pomegranate/35 text-pomegranate hover:bg-pomegranate/[0.12]`}
              >
                {t("metaDelete")}
              </button>
            </li>
          ),
        )}
        {adding ? formRow : null}
        {items.length === 0 && !adding ? (
          <li className="p-6 text-center text-sm text-dust-dim">—</li>
        ) : null}
      </ul>
    </section>
  );
}
