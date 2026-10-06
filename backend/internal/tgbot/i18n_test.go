package tgbot

import "testing"

// allKeys lists every msgKey that must be present in every language —
// keeps the const block and the completeness check independent so a
// forgotten catalog entry fails a test instead of silently falling back
// to English at runtime.
var allKeys = []msgKey{
	kWelcome, kDefaultHint, kNoEvents, kEventsHeader,
	kJoinButton, kOpenWebButton, kEventUnavailable, kJoinedSuccess, kJoinedAlert, kWaitlistedAlert, kJoinWaitlist, kManageAttendance,
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

// allCountKeys is the same contract as allKeys, for the messages whose
// noun agrees with a number and so live in countCatalog instead.
var allCountKeys = []msgKey{kGoingCount, kSpotsLeft}

func TestCatalog_CompleteForEveryLanguage(t *testing.T) {
	for _, l := range []lang{langEn, langRu, langUz} {
		for _, k := range allKeys {
			if _, ok := catalog[l][k]; !ok {
				t.Errorf("catalog[%s] missing key %d", l, k)
			}
		}
	}
}

func TestCountCatalog_CompleteForEveryLanguage(t *testing.T) {
	for _, l := range []lang{langEn, langRu, langUz} {
		for _, k := range allCountKeys {
			forms, ok := countCatalog[l][k]
			if !ok {
				t.Errorf("countCatalog[%s] missing key %d", l, k)
				continue
			}
			for i, f := range forms {
				if f == "" {
					t.Errorf("countCatalog[%s][%d] form %d is empty", l, k, i)
				}
			}
		}
	}
}

// Russian is the reason tc() exists: a single hardcoded form rendered
// "1 участников" for every count. 21 and 111 are the cases a naive
// n==1 / else rule still gets wrong.
func TestTc_RussianAgreement(t *testing.T) {
	cases := map[int]string{
		1: "1 участник", 2: "2 участника", 5: "5 участников",
		11: "11 участников", 21: "21 участник", 111: "111 участников",
	}
	for n, want := range cases {
		if got := tc(langRu, kGoingCount, n); got != want {
			t.Errorf("tc(ru, %d) = %q, want %q", n, got, want)
		}
	}
}

func TestTc_EnglishSingular(t *testing.T) {
	if got := tc(langEn, kSpotsLeft, 1); got != " / 1 spot" {
		t.Errorf("tc(en, 1) = %q, want %q", got, " / 1 spot")
	}
	if got := tc(langEn, kSpotsLeft, 3); got != " / 3 spots" {
		t.Errorf("tc(en, 3) = %q, want %q", got, " / 3 spots")
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
	got := tf(langEn, kLanguageSet, "English")
	if got != "✅ Language set to English." {
		t.Errorf("tf = %q", got)
	}
}

func TestAttendanceMarkupPreservesEventAndLanguage(t *testing.T) {
	for _, language := range []lang{langEn, langRu, langUz} {
		markup := attendanceMarkup("https://meetus.uz", language, 42)
		button := markup.InlineKeyboard[0][0]
		if button.WebApp == nil || button.WebApp.URL != "https://meetus.uz/"+string(language)+"/events/42" {
			t.Fatalf("incorrect attendance destination for %s", language)
		}
		if tText := catalog[language][kManageAttendance]; tText == "" {
			t.Fatal("missing attendance label")
		}
		if tMessage := catalog[language][kWaitlistedAlert]; tMessage == catalog[language][kJoinedAlert] {
			t.Fatal("waitlist must not announce confirmed attendance")
		}
	}
}
