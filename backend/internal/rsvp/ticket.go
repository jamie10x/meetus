package rsvp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"meetus.uz/backend/internal/platform/apperr"
)

// TicketSigner produces and verifies QR payloads of the form
// "<code>.<signature>", where signature = HMAC-SHA256(code, secret).
// The signature lets the check-in endpoint reject forged or mistyped
// codes before touching the database; the code itself is random and
// resolves to a stored ticket.
type TicketSigner struct {
	secret []byte
}

func NewTicketSigner(secret string) *TicketSigner {
	return &TicketSigner{secret: []byte(secret)}
}

func NewTicketCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate ticket code: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *TicketSigner) sign(code string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// QRValue returns the string encoded into the ticket QR code.
func (s *TicketSigner) QRValue(code string) string {
	return code + "." + s.sign(code)
}

// VerifyQR checks the signature and returns the embedded ticket code.
func (s *TicketSigner) VerifyQR(qr string) (string, error) {
	code, sig, ok := strings.Cut(qr, ".")
	if !ok || code == "" || sig == "" {
		return "", apperr.Validation("malformed ticket QR")
	}
	if !hmac.Equal([]byte(s.sign(code)), []byte(sig)) {
		return "", apperr.Validation("invalid ticket signature")
	}
	return code, nil
}
