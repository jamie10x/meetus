# Meetus.uz API Contract

Base URL: `/api`. All responses use the envelope:

```json
{ "data": ... }                                          // success
{ "error": { "code": "...", "message": "..." } }         // failure
```

Error codes: `validation_error` (400), `unauthorized` (401), `forbidden` (403),
`not_found` (404), `conflict` (409), `internal_error` (500).

Authenticated endpoints require `Authorization: Bearer <accessToken>`.

## Auth

### POST /auth/telegram
Body: the raw field map from the Telegram Login Widget callback
(`id`, `first_name`, `last_name?`, `username?`, `photo_url?`, `auth_date`, `hash`).

Response `data`:
```json
{
  "user": { "id": 1, "name": "...", "username": null, "avatarUrl": null,
             "cityId": null, "district": null, "language": "uz", "createdAt": "..." },
  "tokens": { "accessToken": "...", "refreshToken": "...",
               "accessExpiresIn": 900, "refreshExpiresIn": 2592000 }
}
```

### POST /auth/refresh
Body: `{ "refreshToken": "..." }` → `data`: token pair (rotation: old token is revoked).

### POST /auth/logout
Body: `{ "refreshToken": "..." }` → `data`: `{ "loggedOut": true }`. Idempotent.

## Users

### GET /me (auth)
→ `data`: user object (shape above).

### PATCH /me (auth)
Body (all optional): `{ "name", "cityId", "district", "language" }`.
`language` ∈ `uz | ru | en`. → `data`: updated user.

## Meta

### GET /meta/cities · GET /meta/categories
→ `data`: `[{ "id", "slug", "nameUz", "nameRu", "nameEn" }]`
