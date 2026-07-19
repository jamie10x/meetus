package tgbot

import (
	"fmt"

	"meetus.uz/backend/internal/platform/tglang"
)

// lang is one of the three languages the bot supports, mirroring
// users.language ("uz" | "ru" | "en"). Anything unrecognized falls back
// to English.
type lang string

const (
	langUz lang = "uz"
	langRu lang = "ru"
	langEn lang = "en"
)

// normalizeLang converts a raw users.language value into a supported lang.
func normalizeLang(s string) lang {
	switch lang(s) {
	case langUz, langRu:
		return lang(s)
	default:
		return langEn
	}
}

// mapTelegramLangCode guesses a supported language from Telegram's IETF
// language_code (e.g. "ru", "en-US") for a brand-new user, before they've
// set anything explicitly via /language or the web profile page. Shared
// with Mini App login via internal/platform/tglang so both first-contact
// paths agree.
func mapTelegramLangCode(code string) string {
	return tglang.MapCode(code)
}

// langDisplayName is the language's own native name, used as a button
// label regardless of the current message language.
func langDisplayName(l lang) string {
	switch l {
	case langRu:
		return "Русский"
	case langEn:
		return "English"
	default:
		return "Oʻzbekcha"
	}
}

type msgKey int

const (
	kWelcome msgKey = iota
	kDefaultHint
	kNoEvents
	kEventsHeader
	kGoingCount
	kSpotsLeft
	kJoinButton
	kOpenWebButton
	kEventUnavailable
	kJoinedSuccess
	kJoinedAlert
	kLanguagePrompt
	kLanguageSet
	kFeedbackPrompt
	kFeedbackThanks
	kReminder24h
	kReminder1h
	kPlaceOnline
	kPlaceSeeEventPage
	kPlaceInPerson
	kErrAlreadyJoined
	kErrEventFull
	kErrNotOpen
	kErrAlreadyStarted
	kErrGeneric
)

// catalog holds full-message templates per language. Whole sentences are
// translated (not word-by-word) so grammar stays natural; emoji and HTML
// tags live inside the template since they're part of the message layout.
var catalog = map[lang]map[msgKey]string{
	langEn: {
		kWelcome: "👋 Welcome to <b>Meetus.uz</b>, %s!\n\n" +
			"Discover meetups across Uzbekistan and join with one tap.\n\n" +
			"• /events — upcoming events\n" +
			"• /language — change language\n" +
			"• Tickets and profile: %s",
		kDefaultHint:      "Try /events to browse upcoming meetups, or visit %s",
		kNoEvents:         "No upcoming events yet. Check back soon or explore %s",
		kEventsHeader:     "📅 <b>Upcoming events</b>\n\n",
		kGoingCount:       "%d going",
		kSpotsLeft:        " / %d spots",
		kJoinButton:       "✅ Join event",
		kOpenWebButton:    "🌐 Open on Meetus.uz",
		kEventUnavailable: "This event is no longer available.",
		kJoinedSuccess: "✅ You joined! Your QR ticket is ready:\n%s\n\n" +
			"I'll remind you before the event starts.",
		kJoinedAlert:       "You're in! 🎉",
		kLanguagePrompt:    "🌐 Choose your language:",
		kLanguageSet:       "✅ Language set to %s.",
		kFeedbackPrompt:    "🎉 How was <b>%s</b>? Tap a rating below:",
		kFeedbackThanks:    "🙏 Thanks for your feedback!",
		kReminder24h:       "⏰ <b>%s</b> is coming up!\n\n🕐 %s\n📍 %s\n\n🎫 Your ticket: %s",
		kReminder1h:        "⏰ <b>%s</b> starts in about an hour!\n\n🕐 %s\n📍 %s\n\n🎫 Your ticket: %s",
		kPlaceOnline:       "Online",
		kPlaceSeeEventPage: "see event page",
		kPlaceInPerson:     "In person",
		kErrAlreadyJoined:  "You've already joined this event.",
		kErrEventFull:      "Sorry, this event is full.",
		kErrNotOpen:        "This event isn't open for RSVPs.",
		kErrAlreadyStarted: "This event has already started.",
		kErrGeneric:        "Could not join this event.",
	},
	langRu: {
		kWelcome: "👋 Добро пожаловать в <b>Meetus.uz</b>, %s!\n\n" +
			"Находите мероприятия по всему Узбекистану и присоединяйтесь в один клик.\n\n" +
			"• /events — предстоящие мероприятия\n" +
			"• /language — сменить язык\n" +
			"• Билеты и профиль: %s",
		kDefaultHint:      "Введите /events, чтобы посмотреть мероприятия, или откройте %s",
		kNoEvents:         "Пока нет предстоящих мероприятий. Загляните позже или откройте %s",
		kEventsHeader:     "📅 <b>Предстоящие мероприятия</b>\n\n",
		kGoingCount:       "%d участников",
		kSpotsLeft:        " / %d мест",
		kJoinButton:       "✅ Участвовать",
		kOpenWebButton:    "🌐 Открыть на Meetus.uz",
		kEventUnavailable: "Это мероприятие больше недоступно.",
		kJoinedSuccess: "✅ Вы записаны! Ваш QR-билет готов:\n%s\n\n" +
			"Я напомню вам перед началом мероприятия.",
		kJoinedAlert:       "Вы участвуете! 🎉",
		kLanguagePrompt:    "🌐 Выберите язык:",
		kLanguageSet:       "✅ Язык изменён на %s.",
		kFeedbackPrompt:    "🎉 Как прошло <b>%s</b>? Оцените ниже:",
		kFeedbackThanks:    "🙏 Спасибо за отзыв!",
		kReminder24h:       "⏰ <b>%s</b> уже скоро!\n\n🕐 %s\n📍 %s\n\n🎫 Ваш билет: %s",
		kReminder1h:        "⏰ <b>%s</b> начнётся примерно через час!\n\n🕐 %s\n📍 %s\n\n🎫 Ваш билет: %s",
		kPlaceOnline:       "Онлайн",
		kPlaceSeeEventPage: "см. страницу мероприятия",
		kPlaceInPerson:     "Очно",
		kErrAlreadyJoined:  "Вы уже записаны на это мероприятие.",
		kErrEventFull:      "К сожалению, мест больше нет.",
		kErrNotOpen:        "Запись на это мероприятие закрыта.",
		kErrAlreadyStarted: "Это мероприятие уже началось.",
		kErrGeneric:        "Не удалось записаться на мероприятие.",
	},
	langUz: {
		kWelcome: "👋 <b>Meetus.uz</b>ga xush kelibsiz, %s!\n\n" +
			"O'zbekiston bo'ylab tadbirlarni toping va bir tegishda qo'shiling.\n\n" +
			"• /events — yaqinlashib kelayotgan tadbirlar\n" +
			"• /language — tilni o'zgartirish\n" +
			"• Chiptalar va profil: %s",
		kDefaultHint:      "Tadbirlarni ko'rish uchun /events buyrug'ini yuboring yoki %s ga o'ting",
		kNoEvents:         "Hozircha tadbirlar yo'q. Birozdan so'ng qayta tekshiring yoki %s sahifasiga o'ting",
		kEventsHeader:     "📅 <b>Yaqinlashib kelayotgan tadbirlar</b>\n\n",
		kGoingCount:       "%d kishi ishtirok etmoqda",
		kSpotsLeft:        " / %d joy",
		kJoinButton:       "✅ Qatnashish",
		kOpenWebButton:    "🌐 Meetus.uz'da ochish",
		kEventUnavailable: "Bu tadbir endi mavjud emas.",
		kJoinedSuccess: "✅ Siz qatnashuvchi sifatida qo'shildingiz! QR-chiptangiz tayyor:\n%s\n\n" +
			"Tadbir boshlanishidan oldin sizga eslataman.",
		kJoinedAlert:       "Siz ro'yxatdasiz! 🎉",
		kLanguagePrompt:    "🌐 Tilni tanlang:",
		kLanguageSet:       "✅ Til %s qilib o'zgartirildi.",
		kFeedbackPrompt:    "🎉 <b>%s</b> qanday o'tdi? Bahoni tanlang:",
		kFeedbackThanks:    "🙏 Fikringiz uchun rahmat!",
		kReminder24h:       "⏰ <b>%s</b> tez orada boshlanadi!\n\n🕐 %s\n📍 %s\n\n🎫 Chiptangiz: %s",
		kReminder1h:        "⏰ <b>%s</b> taxminan bir soatdan so'ng boshlanadi!\n\n🕐 %s\n📍 %s\n\n🎫 Chiptangiz: %s",
		kPlaceOnline:       "Onlayn",
		kPlaceSeeEventPage: "tadbir sahifasiga qarang",
		kPlaceInPerson:     "Yuzma-yuz",
		kErrAlreadyJoined:  "Siz allaqachon bu tadbirga qo'shilgansiz.",
		kErrEventFull:      "Afsuski, joylar tugadi.",
		kErrNotOpen:        "Bu tadbirga ro'yxatdan o'tish yopiq.",
		kErrAlreadyStarted: "Bu tadbir allaqachon boshlangan.",
		kErrGeneric:        "Tadbirga qo'shilib bo'lmadi.",
	},
}

// t looks up a translated template, falling back to English if a language
// or key is somehow missing (defensive — i18n_test.go asserts completeness).
func t(l lang, k msgKey) string {
	if s, ok := catalog[l][k]; ok {
		return s
	}
	return catalog[langEn][k]
}

// tf formats a translated template with args.
func tf(l lang, k msgKey, args ...any) string {
	return fmt.Sprintf(t(l, k), args...)
}
