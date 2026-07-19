package tgbot

import "testing"

func TestParseFeedbackCallback(t *testing.T) {
	tests := []struct {
		data       string
		wantEvent  int64
		wantRating int
		wantOK     bool
	}{
		{"fb:42:5", 42, 5, true},
		{"fb:1:1", 1, 1, true},
		{"fb:42", 0, 0, false},
		{"fb:abc:5", 0, 0, false},
		{"fb:42:abc", 0, 0, false},
		{"fb:", 0, 0, false},
	}
	for _, tt := range tests {
		gotEvent, gotRating, ok := parseFeedbackCallback(tt.data)
		if ok != tt.wantOK {
			t.Errorf("parseFeedbackCallback(%q) ok = %v, want %v", tt.data, ok, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if gotEvent != tt.wantEvent || gotRating != tt.wantRating {
			t.Errorf("parseFeedbackCallback(%q) = (%d, %d), want (%d, %d)",
				tt.data, gotEvent, gotRating, tt.wantEvent, tt.wantRating)
		}
	}
}
