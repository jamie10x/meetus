export type User = {
  id: number;
  name: string;
  username: string | null;
  avatarUrl: string | null;
  cityId: number | null;
  district: string | null;
  language: "uz" | "ru" | "en";
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

/** Raw payload delivered by the Telegram Login Widget callback. */
export type TelegramAuthFields = Record<string, string>;
