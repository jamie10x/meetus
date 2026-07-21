export type User = {
  id: number;
  name: string;
  username: string | null;
  avatarUrl: string | null;
  cityId: number | null;
  district: string | null;
  language: "uz" | "ru" | "en";
  isAdmin: boolean;
  createdAt: string;
};

export type TokenPair = {
  accessToken: string;
  refreshToken: string;
  accessExpiresIn: number;
  refreshExpiresIn: number;
};

export type LoginResult = {
  user: User;
  tokens: TokenPair;
};

export type MetaItem = {
  id: number;
  slug: string;
  nameUz: string;
  nameRu: string;
  nameEn: string;
};

/** Picks the city/category display name matching the active locale. */
export function metaName(item: MetaItem, locale: string): string {
  if (locale === "ru") return item.nameRu;
  if (locale === "uz") return item.nameUz;
  return item.nameEn;
}

/** Raw payload delivered by the Telegram Login Widget callback. */
export type TelegramAuthFields = Record<string, string>;

export type Channel = {
  id: number;
  chatTitle: string;
  language: string | null;
  connectedAt: string;
};

export type Organizer = {
  id: number;
  displayName: string;
  bio: string | null;
  avatarUrl: string | null;
  isVerified: boolean;
  createdAt: string;
};

export type EventStatus = "draft" | "published" | "canceled" | "finished";

export type EventItem = {
  id: number;
  organizerId: number;
  organizerName: string;
  organizerVerified: boolean;
  title: string;
  description: string;
  categoryId: number;
  categorySlug: string;
  cityId: number | null;
  citySlug: string | null;
  district: string | null;
  locationName: string | null;
  address: string | null;
  lat: number | null;
  lng: number | null;
  isOnline: boolean;
  startsAt: string;
  endsAt: string | null;
  capacity: number | null;
  coverUrl: string | null;
  status: EventStatus;
  visibility: "public" | "unlisted";
  seriesId: number | null;
  goingCount: number;
  createdAt: string;
};

export type TrendingEventItem = EventItem & {
  recentGoing: number;
};

export type EventInput = {
  title: string;
  description: string;
  categoryId: number;
  cityId: number | null;
  district: string | null;
  locationName: string | null;
  address: string | null;
  lat: number | null;
  lng: number | null;
  isOnline: boolean;
  startsAt: string;
  endsAt: string | null;
  capacity: number | null;
  coverUrl: string | null;
  /** Additional weekly occurrences to create alongside this one (create only). */
  recurWeeks?: number | null;
};
