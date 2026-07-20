package config

import "testing"

func TestLoad_OfficialChannel(t *testing.T) {
	t.Run("unset means disabled", func(t *testing.T) {
		t.Setenv("TELEGRAM_OFFICIAL_CHANNEL_ID", "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OfficialChannelID != 0 {
			t.Errorf("OfficialChannelID = %d, want 0 (disabled)", cfg.OfficialChannelID)
		}
		if cfg.OfficialChannelLanguage != "uz" {
			t.Errorf("OfficialChannelLanguage = %q, want default %q", cfg.OfficialChannelLanguage, "uz")
		}
	})

	t.Run("valid chat id parses", func(t *testing.T) {
		t.Setenv("TELEGRAM_OFFICIAL_CHANNEL_ID", "-1001234567890")
		t.Setenv("TELEGRAM_OFFICIAL_CHANNEL_LANGUAGE", "ru")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OfficialChannelID != -1001234567890 {
			t.Errorf("OfficialChannelID = %d, want -1001234567890", cfg.OfficialChannelID)
		}
		if cfg.OfficialChannelLanguage != "ru" {
			t.Errorf("OfficialChannelLanguage = %q, want %q", cfg.OfficialChannelLanguage, "ru")
		}
	})

	t.Run("garbage value errors instead of silently disabling", func(t *testing.T) {
		t.Setenv("TELEGRAM_OFFICIAL_CHANNEL_ID", "not-a-number")
		if _, err := Load(); err == nil {
			t.Fatal("expected an error for a non-numeric TELEGRAM_OFFICIAL_CHANNEL_ID, got nil")
		}
	})
}
