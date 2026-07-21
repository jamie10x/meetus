package tgbot

import "testing"

// allKeys lists every msgKey that must be present in every language —
// keeps the const block and the completeness check independent so a
// forgotten catalog entry fails a test instead of silently falling back
// to English at runtime.
var allKeys = []msgKey{
	kWelcome, kDefaultHint, kNoEvents, kEventsHeader, kGoingCount, kSpotsLeft,
	kJoinButton, kOpenWebButton, kEventUnavailable, kJoinedSuccess, kJoinedAlert,
	kLanguagePrompt, kLanguageSet, kFeedbackPrompt, kFeedbackThanks,
	kFeedbackCommentPrompt, kFeedbackCommentThanks, kSkipButton,
	kReminder24h, kReminder1h, kPlaceOnline, kPlaceSeeEventPage, kPlaceInPerson,
	kErrAlreadyJoined, kErrEventFull, kErrNotOpen, kErrAlreadyStarted, kErrGeneric,
	kChannelConnected, kChannelConnectNeedsOrganizer, kAnnouncementCta,
	kTicketCaption, kNoUpcomingTickets, kWaitlisted, kWaitlistPromoted,
	kMuted, kUnmuted, kDigestOn, kDigestOff, kDigestHeader,
	kNearbyPrompt, kShareLocationButton, kNearbyHeader, kNearbyEmpty,
	kGroupSubscribed,
}

func TestCatalog_CompleteForEveryLanguage(t *testing.T) {
	for _, l := range []lang{langEn, langRu, langUz} {
		for _, k := range allKeys {
			if _, ok := catalog[l][k]; !ok {
				t.Errorf("catalog[%s] missing key %d", l, k)
			}
		}
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]lang{
		"uz": langUz, "ru": langRu, "en": langEn,
		"":   langEn,
		"fr": langEn,
		"UZ": langEn, // case-sensitive by design — DB values are always lowercase
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapTelegramLangCode(t *testing.T) {
	cases := map[string]string{
		"ru":    "ru",
		"ru-RU": "ru",
		"en":    "en",
		"en-US": "en",
		"uz":    "uz",
		"uz-UZ": "uz",
		"fr":    "uz", // unsupported code defaults to uz (majority audience)
		"":      "uz",
	}
	for in, want := range cases {
		if got := mapTelegramLangCode(in); got != want {
			t.Errorf("mapTelegramLangCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTf_Formats(t *testing.T) {
	got := tf(langEn, kGoingCount, 5)
	if got != "5 going" {
		t.Errorf("tf = %q", got)
	}
}
