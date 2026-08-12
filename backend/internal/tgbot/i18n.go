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
	kFeedbackCommentPrompt
	kFeedbackCommentThanks
	kSkipButton
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
	kChannelConnected
	kChannelConnectNeedsOrganizer
	kAnnouncementCta
	kTicketCaption
	kNoUpcomingTickets
	kWaitlisted
	kWaitlistPromoted
	kMuted
	kUnmuted
	kDigestOn
	kDigestOff
	kDigestHeader
	kNearbyPrompt
	kShareLocationButton
	kNearbyHeader
	kNearbyEmpty
	kGroupSubscribed
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
		kJoinButton:       "✅ Join event",
		kOpenWebButton:    "🌐 Open on Meetus.uz",
		kEventUnavailable: "This event is no longer available.",
		kJoinedSuccess: "✅ You're in! Your QR ticket is below.\n\n" +
			"I'll remind you before the event starts.",
		kJoinedAlert:           "You're in! 🎉",
		kLanguagePrompt:        "🌐 Choose your language:",
		kLanguageSet:           "✅ Language set to %s.",
		kFeedbackPrompt:        "🎉 How was <b>%s</b>? Tap a rating below:",
		kFeedbackThanks:        "🙏 Thanks for your feedback!",
		kFeedbackCommentPrompt: "Want to add a comment? Reply with a message, or tap Skip.",
		kFeedbackCommentThanks: "🙏 Thanks — your comment was added!",
		kSkipButton:            "Skip",
		kReminder24h:           "⏰ <b>%s</b> is coming up!\n\n🕐 %s\n📍 %s\n\n🎫 Your ticket: %s",
		kReminder1h:            "⏰ <b>%s</b> starts in about an hour!\n\n🕐 %s\n📍 %s\n\n🎫 Your ticket: %s",
		kPlaceOnline:           "Online",
		kPlaceSeeEventPage:     "see event page",
		kPlaceInPerson:         "In person",
		kErrAlreadyJoined:      "You've already joined this event.",
		kErrEventFull:          "Sorry, this event is full.",
		kErrNotOpen:            "This event isn't open for RSVPs.",
		kErrAlreadyStarted:     "This event has already started.",
		kErrGeneric:            "Could not join this event.",
		kChannelConnected: "✅ Channel <b>%s</b> connected! You can now send event " +
			"announcements to it from your Meetus.uz organizer dashboard.",
		kChannelConnectNeedsOrganizer: "This channel needs an owner with a Meetus.uz organizer " +
			"profile. Sign in and create one first, then add me as admin here again.",
		kAnnouncementCta:   "🎟️ View & join",
		kTicketCaption:     "🎟️ <b>%s</b>\n\n🕐 %s\n📍 %s\n\nShow this QR at the entrance.",
		kNoUpcomingTickets: "You don't have any upcoming tickets yet. Try /events to find something.",
		kWaitlisted: "You're on the waitlist 📋 This event is full right now — " +
			"I'll message you the moment a spot opens up.",
		kWaitlistPromoted:    "🎉 A spot opened up for <b>%s</b> and you're in! Your QR ticket is below.",
		kMuted:               "🔕 Reminders and feedback prompts are now off. Send /mute again anytime to turn them back on.",
		kUnmuted:             "🔔 Reminders and feedback prompts are back on.",
		kDigestOn:            "🔔 Weekly digest is on — I'll send what's coming up each Monday morning.",
		kDigestOff:           "🔕 Weekly digest is off. Send /digest anytime to turn it back on.",
		kDigestHeader:        "📅 <b>This week on Meetus.uz</b>\n\n",
		kNearbyPrompt:        "Share your location and I'll find events happening near you.",
		kShareLocationButton: "📍 Share my location",
		kNearbyHeader:        "📍 <b>Events near you</b>\n\n",
		kNearbyEmpty:         "No upcoming events nearby yet. Try /events to browse everything.",
		kGroupSubscribed:     "✅ This group is now subscribed to Meetus.uz event announcements.",
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
		kJoinButton:       "✅ Участвовать",
		kOpenWebButton:    "🌐 Открыть на Meetus.uz",
		kEventUnavailable: "Это мероприятие больше недоступно.",
		kJoinedSuccess: "✅ Вы участвуете! Ваш QR-билет ниже.\n\n" +
			"Я напомню вам перед началом мероприятия.",
		kJoinedAlert:           "Вы участвуете! 🎉",
		kLanguagePrompt:        "🌐 Выберите язык:",
		kLanguageSet:           "✅ Язык изменён на %s.",
		kFeedbackPrompt:        "🎉 Как прошло <b>%s</b>? Оцените ниже:",
		kFeedbackThanks:        "🙏 Спасибо за отзыв!",
		kFeedbackCommentPrompt: "Хотите добавить комментарий? Ответьте сообщением или нажмите «Пропустить».",
		kFeedbackCommentThanks: "🙏 Спасибо — комментарий добавлен!",
		kSkipButton:            "Пропустить",
		kReminder24h:           "⏰ <b>%s</b> уже скоро!\n\n🕐 %s\n📍 %s\n\n🎫 Ваш билет: %s",
		kReminder1h:            "⏰ <b>%s</b> начнётся примерно через час!\n\n🕐 %s\n📍 %s\n\n🎫 Ваш билет: %s",
		kPlaceOnline:           "Онлайн",
		kPlaceSeeEventPage:     "см. страницу мероприятия",
		kPlaceInPerson:         "Очно",
		kErrAlreadyJoined:      "Вы уже записаны на это мероприятие.",
		kErrEventFull:          "К сожалению, мест больше нет.",
		kErrNotOpen:            "Запись на это мероприятие закрыта.",
		kErrAlreadyStarted:     "Это мероприятие уже началось.",
		kErrGeneric:            "Не удалось записаться на мероприятие.",
		kChannelConnected: "✅ Канал <b>%s</b> подключён! Теперь вы можете отправлять " +
			"анонсы мероприятий в него из панели организатора на Meetus.uz.",
		kChannelConnectNeedsOrganizer: "У этого канала должен быть владелец с профилем " +
			"организатора на Meetus.uz. Войдите и сначала создайте профиль, затем добавьте " +
			"меня сюда администратором ещё раз.",
		kAnnouncementCta:   "🎟️ Смотреть и участвовать",
		kTicketCaption:     "🎟️ <b>%s</b>\n\n🕐 %s\n📍 %s\n\nПокажите этот QR-код на входе.",
		kNoUpcomingTickets: "У вас пока нет предстоящих билетов. Попробуйте /events, чтобы найти что-нибудь.",
		kWaitlisted: "Вы в списке ожидания 📋 Сейчас на мероприятии нет мест — " +
			"я напишу вам, как только освободится место.",
		kWaitlistPromoted:    "🎉 Освободилось место на <b>%s</b>, и вы участвуете! Ваш QR-билет ниже.",
		kMuted:               "🔕 Напоминания и запросы отзывов отключены. Отправьте /mute снова, чтобы включить их.",
		kUnmuted:             "🔔 Напоминания и запросы отзывов снова включены.",
		kDigestOn:            "🔔 Еженедельная подборка включена — буду присылать её каждый понедельник утром.",
		kDigestOff:           "🔕 Еженедельная подборка отключена. Отправьте /digest, чтобы включить снова.",
		kDigestHeader:        "📅 <b>На этой неделе на Meetus.uz</b>\n\n",
		kNearbyPrompt:        "Поделитесь геолокацией, и я найду мероприятия рядом с вами.",
		kShareLocationButton: "📍 Поделиться геолокацией",
		kNearbyHeader:        "📍 <b>Мероприятия рядом с вами</b>\n\n",
		kNearbyEmpty:         "Пока нет предстоящих мероприятий поблизости. Попробуйте /events, чтобы посмотреть все.",
		kGroupSubscribed:     "✅ Эта группа теперь подписана на анонсы мероприятий Meetus.uz.",
	},
	langUz: {
		kWelcome: "👋 <b>Meetus.uz</b>ga xush kelibsiz, %s!\n\n" +
			"Oʻzbekiston boʻylab tadbirlarni toping va bir tegishda qoʻshiling.\n\n" +
			"• /events — yaqinlashib kelayotgan tadbirlar\n" +
			"• /language — tilni oʻzgartirish\n" +
			"• Chiptalar va profil: %s",
		kDefaultHint:      "Tadbirlarni koʻrish uchun /events buyrugʻini yuboring yoki %s ga oʻting",
		kNoEvents:         "Hozircha tadbirlar yoʻq. Birozdan soʻng qayta tekshiring yoki %s sahifasiga oʻting",
		kEventsHeader:     "📅 <b>Yaqinlashib kelayotgan tadbirlar</b>\n\n",
		kJoinButton:       "✅ Qatnashish",
		kOpenWebButton:    "🌐 Meetus.uzda ochish",
		kEventUnavailable: "Bu tadbir endi mavjud emas.",
		kJoinedSuccess: "✅ Siz qatnashuvchisiz! QR-chiptangiz quyida.\n\n" +
			"Tadbir boshlanishidan oldin sizga eslataman.",
		kJoinedAlert:           "Siz roʻyxatdasiz! 🎉",
		kLanguagePrompt:        "🌐 Tilni tanlang:",
		kLanguageSet:           "✅ Til %s qilib oʻzgartirildi.",
		kFeedbackPrompt:        "🎉 <b>%s</b> qanday oʻtdi? Bahoni tanlang:",
		kFeedbackThanks:        "🙏 Fikringiz uchun rahmat!",
		kFeedbackCommentPrompt: "Izoh qoldirmoqchimisiz? Xabar yozing yoki \"Oʻtkazib yuborish\"ni bosing.",
		kFeedbackCommentThanks: "🙏 Rahmat — izohingiz qoʻshildi!",
		kSkipButton:            "Oʻtkazib yuborish",
		kReminder24h:           "⏰ <b>%s</b> tez orada boshlanadi!\n\n🕐 %s\n📍 %s\n\n🎫 Chiptangiz: %s",
		kReminder1h:            "⏰ <b>%s</b> taxminan bir soatdan soʻng boshlanadi!\n\n🕐 %s\n📍 %s\n\n🎫 Chiptangiz: %s",
		kPlaceOnline:           "Onlayn",
		kPlaceSeeEventPage:     "tadbir sahifasiga qarang",
		kPlaceInPerson:         "Yuzma-yuz",
		kErrAlreadyJoined:      "Siz allaqachon bu tadbirga qoʻshilgansiz.",
		kErrEventFull:          "Afsuski, joylar tugadi.",
		kErrNotOpen:            "Bu tadbirga roʻyxatdan oʻtish yopiq.",
		kErrAlreadyStarted:     "Bu tadbir allaqachon boshlangan.",
		kErrGeneric:            "Tadbirga qoʻshilib boʻlmadi.",
		kChannelConnected: "✅ <b>%s</b> kanali ulandi! Endi Meetus.uz tashkilotchi " +
			"panelidan bu kanalga tadbir eʼlonlarini yuborishingiz mumkin.",
		kChannelConnectNeedsOrganizer: "Bu kanalning Meetus.uzda tashkilotchi profiliga ega " +
			"egasi boʻlishi kerak. Avval kiring va profil yarating, soʻng meni yana admin " +
			"qilib qoʻshing.",
		kAnnouncementCta:   "🎟️ Koʻrish va qatnashish",
		kTicketCaption:     "🎟️ <b>%s</b>\n\n🕐 %s\n📍 %s\n\nKirishda ushbu QR-kodni koʻrsating.",
		kNoUpcomingTickets: "Sizda hozircha yaqinlashib kelayotgan chiptalar yoʻq. /events buyrugʻini sinab koʻring.",
		kWaitlisted: "Siz kutish roʻyxatidasiz 📋 Hozircha bu tadbirda joy yoʻq — " +
			"joy boʻshashi bilanoq sizga xabar beraman.",
		kWaitlistPromoted:    "🎉 <b>%s</b> uchun joy boʻshadi va siz qatnashuvchisiz! QR-chiptangiz quyida.",
		kMuted:               "🔕 Eslatmalar va fikr-mulohaza soʻrovlari endi oʻchirilgan. Yoqish uchun istalgan vaqtda /mute yuboring.",
		kUnmuted:             "🔔 Eslatmalar va fikr-mulohaza soʻrovlari qayta yoqildi.",
		kDigestOn:            "🔔 Haftalik xulosa yoqildi — har dushanba ertalab yuboraman.",
		kDigestOff:           "🔕 Haftalik xulosa oʻchirildi. Qayta yoqish uchun /digest yuboring.",
		kDigestHeader:        "📅 <b>Bu hafta Meetus.uzda</b>\n\n",
		kNearbyPrompt:        "Manzilingizni ulashing, men yaqin atrofdagi tadbirlarni topaman.",
		kShareLocationButton: "📍 Manzilimni ulashish",
		kNearbyHeader:        "📍 <b>Yaqin atrofdagi tadbirlar</b>\n\n",
		kNearbyEmpty:         "Yaqin atrofda hozircha tadbirlar yoʻq. Barchasini koʻrish uchun /events buyrugʻini sinab koʻring.",
		kGroupSubscribed:     "✅ Bu guruh endi Meetus.uz tadbir eʼlonlariga obuna boʻldi.",
	},
}

// pluralIndex picks which of (one, few, many) a count takes. Russian
// agrees three ways — 1 участник / 2 участника / 5 участников — so a
// single hardcoded form was wrong for most numbers ("1 участников").
// English agrees two ways; Uzbek not at all, since a numeral never
// changes the noun after it.
func pluralIndex(l lang, n int) int {
	if n < 0 {
		n = -n
	}
	switch l {
	case langRu:
		switch {
		case n%10 == 1 && n%100 != 11:
			return 0
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			return 1
		default:
			return 2
		}
	case langUz:
		return 0
	default:
		if n == 1 {
			return 0
		}
		return 2
	}
}

// countCatalog holds messages whose noun agrees with a number, as
// (one, few, many). Uzbek fills only index 0 meaningfully; English uses
// 0 and 2. Kept separate from catalog so tc() can't be called on a
// template that has no plural forms, and vice versa.
var countCatalog = map[lang]map[msgKey][3]string{
	langEn: {
		kGoingCount: {"%d going", "%d going", "%d going"},
		kSpotsLeft:  {" / %d spot", " / %d spots", " / %d spots"},
	},
	langRu: {
		kGoingCount: {"%d участник", "%d участника", "%d участников"},
		kSpotsLeft:  {" / %d место", " / %d места", " / %d мест"},
	},
	langUz: {
		kGoingCount: {"%d kishi qatnashadi", "%d kishi qatnashadi", "%d kishi qatnashadi"},
		kSpotsLeft:  {" / %d ta joy", " / %d ta joy", " / %d ta joy"},
	},
}

// tc formats a count-agreeing message in the right plural form.
func tc(l lang, k msgKey, n int) string {
	forms, ok := countCatalog[l][k]
	if !ok {
		forms = countCatalog[langEn][k]
	}
	return fmt.Sprintf(forms[pluralIndex(l, n)], n)
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
