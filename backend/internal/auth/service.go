package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/tglang"
	"meetus.uz/backend/internal/user"
)

type Service struct {
	users           *user.Repository
	repo            *Repository
	tokens          *authn.TokenManager
	botToken        string
	refreshTokenTTL time.Duration
	now             func() time.Time
}

func NewService(users *user.Repository, repo *Repository, tokens *authn.TokenManager, botToken string, refreshTokenTTL time.Duration) *Service {
	return &Service{
		users:           users,
		repo:            repo,
		tokens:          tokens,
		botToken:        botToken,
		refreshTokenTTL: refreshTokenTTL,
		now:             time.Now,
	}
}

type TokenPair struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	AccessExpiresIn  int64  `json:"accessExpiresIn"`
	RefreshExpiresIn int64  `json:"refreshExpiresIn"`
}

type LoginResult struct {
	User   user.DTO  `json:"user"`
	Tokens TokenPair `json:"tokens"`
}

// LoginWithTelegram verifies the Telegram Login Widget payload, upserts the
// user, and issues a fresh token pair.
func (s *Service) LoginWithTelegram(ctx context.Context, fields map[string]string) (*LoginResult, error) {
	tu, err := VerifyTelegramLogin(fields, s.botToken, s.now())
	if err != nil {
		return nil, err
	}
	// The Login Widget payload carries no language hint; "uz" matches the
	// column's own default.
	return s.loginTelegramUser(ctx, tu, "uz")
}

// LoginWithMiniApp verifies initData from a Telegram Mini App launch,
// upserts the user, and issues a fresh token pair. Unlike the Login
// Widget, initData does carry a language hint (the Telegram client's own
// language_code), so a brand-new Mini App user gets a better first guess.
func (s *Service) LoginWithMiniApp(ctx context.Context, initData string) (*LoginResult, error) {
	tu, err := VerifyMiniAppInitData(initData, s.botToken, s.now())
	if err != nil {
		return nil, err
	}
	return s.loginTelegramUser(ctx, tu, tglang.MapCode(tu.LanguageCode))
}

// loginTelegramUser is the shared tail of both login flows: upsert, ban
// check, issue tokens. defaultLanguage seeds a brand-new user's language
// column (ignored for existing users — see user.TelegramProfile.Language).
func (s *Service) loginTelegramUser(ctx context.Context, tu *TelegramUser, defaultLanguage string) (*LoginResult, error) {
	u, err := s.users.UpsertTelegramUser(ctx, user.TelegramProfile{
		TelegramID: tu.ID,
		Name:       tu.DisplayName(),
		Username:   tu.Username,
		AvatarURL:  tu.PhotoURL,
		Language:   defaultLanguage,
	})
	if err != nil {
		return nil, err
	}
	if u.IsBanned {
		return nil, apperr.Forbidden("account is banned")
	}

	pair, err := s.issuePair(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u.ToDTO(), Tokens: *pair}, nil
}

// Refresh rotates a valid refresh token: the old token is revoked and a new
// pair is issued. A revoked or expired token is rejected.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	stored, err := s.repo.GetRefreshToken(ctx, hashToken(rawRefreshToken))
	if err != nil {
		return nil, err
	}
	if stored.RevokedAt != nil {
		return nil, apperr.Unauthorized("refresh token revoked")
	}
	if s.now().After(stored.ExpiresAt) {
		return nil, apperr.Unauthorized("refresh token expired")
	}

	// Reject refresh for banned/deleted users before issuing new tokens.
	u, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, apperr.Unauthorized("account unavailable")
	}
	if u.IsBanned {
		return nil, apperr.Forbidden("account is banned")
	}

	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return nil, err
	}
	return s.issuePair(ctx, stored.UserID)
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	stored, err := s.repo.GetRefreshToken(ctx, hashToken(rawRefreshToken))
	if err != nil {
		// Unknown token: logout is idempotent.
		return nil
	}
	return s.repo.RevokeRefreshToken(ctx, stored.ID)
}

func (s *Service) issuePair(ctx context.Context, userID int64) (*TokenPair, error) {
	now := s.now()

	access, err := s.tokens.IssueAccess(userID, now)
	if err != nil {
		return nil, err
	}

	raw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(s.refreshTokenTTL)
	if err := s.repo.StoreRefreshToken(ctx, userID, hashToken(raw), expiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:      access,
		RefreshToken:     raw,
		AccessExpiresIn:  int64(s.tokens.AccessTTL().Seconds()),
		RefreshExpiresIn: int64(s.refreshTokenTTL.Seconds()),
	}, nil
}

func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken stores only a digest of the refresh token so a database leak
// does not expose usable tokens.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
