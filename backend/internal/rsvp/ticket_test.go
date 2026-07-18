package rsvp

import (
	"strings"
	"testing"
)

func TestTicketSigner_RoundTrip(t *testing.T) {
	s := NewTicketSigner("secret")
	code, err := NewTicketCode()
	if err != nil {
		t.Fatal(err)
	}
	qr := s.QRValue(code)
	got, err := s.VerifyQR(qr)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != code {
		t.Errorf("got %q, want %q", got, code)
	}
}

func TestTicketSigner_Tampered(t *testing.T) {
	s := NewTicketSigner("secret")
	code, _ := NewTicketCode()
	qr := s.QRValue(code)

	tampered := strings.Replace(qr, code[:4], "0000", 1)
	if tampered == qr {
		t.Skip("random code started with 0000")
	}
	if _, err := s.VerifyQR(tampered); err == nil {
		t.Fatal("expected error for tampered QR")
	}
}

func TestTicketSigner_WrongSecret(t *testing.T) {
	a := NewTicketSigner("secret-a")
	b := NewTicketSigner("secret-b")
	code, _ := NewTicketCode()
	if _, err := b.VerifyQR(a.QRValue(code)); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestTicketSigner_Malformed(t *testing.T) {
	s := NewTicketSigner("secret")
	for _, qr := range []string{"", "nodot", ".", "abc.", ".def"} {
		if _, err := s.VerifyQR(qr); err == nil {
			t.Fatalf("expected error for %q", qr)
		}
	}
}
