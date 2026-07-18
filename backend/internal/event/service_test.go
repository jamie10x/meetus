package event

import (
	"strings"
	"testing"
	"time"
)

func validInput() Input {
	cityID := int32(1)
	return Input{
		Title:      "Go Meetup Tashkent",
		CategoryID: 1,
		CityID:     &cityID,
		StartsAt:   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
	}
}

func TestValidate(t *testing.T) {
	s := NewService(nil)

	tests := []struct {
		name    string
		mutate  func(*Input)
		wantErr string // substring of expected error; empty = valid
	}{
		{"valid minimal", func(in *Input) {}, ""},
		{"bad startsAt", func(in *Input) { in.StartsAt = "tomorrow" }, "RFC3339"},
		{"endsAt before startsAt", func(in *Input) {
			e := time.Now().Add(time.Hour).Format(time.RFC3339)
			in.EndsAt = &e
		}, "after startsAt"},
		{"zero capacity", func(in *Input) { c := int32(0); in.Capacity = &c }, "positive"},
		{"negative capacity", func(in *Input) { c := int32(-5); in.Capacity = &c }, "positive"},
		{"offline without city", func(in *Input) { in.CityID = nil }, "cityId"},
		{"online without city is fine", func(in *Input) {
			in.CityID = nil
			in.IsOnline = true
		}, ""},
		{"bad visibility", func(in *Input) { v := "secret"; in.Visibility = &v }, "visibility"},
		{"unlisted visibility", func(in *Input) { v := "unlisted"; in.Visibility = &v }, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			_, err := s.validate(in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
