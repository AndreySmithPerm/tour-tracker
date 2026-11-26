package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tb "gopkg.in/telebot.v3"
	"github.com/travel-to-tour/tour-tracker/internal/domain"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// bot phases
const (
	phaseOriginCity  = "origin_city"
	phaseDestination = "destination"
	phaseDateFrom    = "date_from"
	phaseNights      = "nights"
	phaseMaxPrice    = "max_price"
)

// Bot wraps the telebot bot with our logic
type Bot struct {
	teleBot  *tb.Bot
	storage  *storage.Storage
	logger   *log.Logger
	phaseMap map[int64]string // user_id -> current phase
	// Store intermediate values for subscription creation
	subData map[int64]*subscriptionData
}

type subscriptionData struct {
	OriginCity  string
	Destination string
	DateFrom    string
	Nights      string
}

// NewBot creates a new Telegram bot instance
func NewBot(token string, s *storage.Storage) (*Bot, error) {
	b, err := tb.NewBot(tb.Settings{
		Token: token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create telebot: %w", err)
	}

	bot := &Bot{
		teleBot:  b,
		storage:  s,
		logger:   log.Default(),
		phaseMap: make(map[int64]string),
		subData:  make(map[int64]*subscriptionData),
	}

	bot.setupHandlers()
	return bot, nil
}

func (b *Bot) setupHandlers() {
	// Welcome message on new member
	b.teleBot.Handle("/start", func(c tb.Context) error {
		return c.Send(
			"🌴 <b>Tour Tracker</b> — найдём лучшие цены на туры!\n\n"+
				"Мы мониторим цены от ведущих туроператоров и уведомляем, когда цена падает.\n\n"+
				"<b>Команды:</b>\n"+
				"➕ /add — добавить подписку на отслеживание\n"+
				"📋 /subs — список активных подписок\n"+
				"❌ /remove — удалить подписку\n"+
				"❓ /help — справка\n\n"+
				"Или подпишитесь на сайте: gid-ok.ru 🌐",
		&tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{
				{
					{Text: "➕ Добавить", Data: "add_start"},
					{Text: "📋 Подписки", Data: "subs_list"},
				},
			},
		},
		)
	})

	b.teleBot.Handle("add_start", func(c tb.Context) error {
		return b.handleAddStart(c)
	})

	b.teleBot.Handle("subs_list", func(c tb.Context) error {
		return b.handleListSubs(c)
	})

	b.teleBot.Handle("/help", func(c tb.Context) error {
		return c.Send(
			"<b>📖 Справка по командам</b>\n\n"+
				"➕ <b>/add</b> — добавить подписку на отслеживание туров\n\n"+
				"📋 <b>/subs</b> — показать список активных подписок\n\n"+
				"❌ <b>/remove &lt;номер&gt;</b> — удалить подписку по номеру\n\n"+
				"⏹ <b>/cancel</b> — отменить добавление подписки\n\n"+
				"🌐 Подписаться на сайте: gid-ok.ru",
		)
	})

	b.teleBot.Handle("/add", func(c tb.Context) error {
		return b.handleAddStart(c)
	})

	b.teleBot.Handle("/subs", func(c tb.Context) error {
		return b.handleListSubs(c)
	})

	b.teleBot.Handle("/remove", func(c tb.Context) error {
		return b.handleRemoveSub(c)
	})

	// Handle text messages for multi-step add flow
	b.teleBot.Handle(tb.OnText, func(c tb.Context) error {
		userID := c.Sender().ID
		phase := b.phaseMap[userID]

		if phase == "" {
			return nil
		}

		return b.handleAddStep(c, userID, phase)
	})

	// Cancel command
	b.teleBot.Handle("/cancel", func(c tb.Context) error {
		userID := c.Sender().ID
		delete(b.phaseMap, userID)
		delete(b.subData, userID)
		return c.Send("⏹ Добавление отменено.")
	})
}

func (b *Bot) handleAddStart(c tb.Context) error {
	userID := c.Sender().ID
	b.phaseMap[userID] = phaseOriginCity
	b.subData[userID] = &subscriptionData{}
	return c.Send(
		"🌍 <b>Шаг 1/5</b>\n\n"+
			"Введите город вылета (например: Москва, Пермь, Санкт-Петербург):",
	)
}

func (b *Bot) handleAddStep(c tb.Context, userID int64, phase string) error {
	text := strings.TrimSpace(c.Text())
	data := b.subData[userID]
	if data == nil {
		data = &subscriptionData{}
		b.subData[userID] = data
	}

	switch phase {
	case phaseOriginCity:
		if len(text) < 2 {
			return c.Send("⚠️ Город слишком короткий. Введите город вылета:")
		}
		data.OriginCity = text
		b.phaseMap[userID] = phaseDestination
		return c.Send(
			"🏖 <b>Шаг 2/5</b>\n\n"+
				"Введите направление (страну/город):\n"+
				"Примеры: Турция, Таиланд, ОАЭ, Египет, Сочи",
		)

	case phaseDestination:
		if len(text) < 2 {
			return c.Send("⚠️ Направление слишком короткое. Введите страну или город:")
		}
		data.Destination = text
		b.phaseMap[userID] = phaseDateFrom
		return c.Send(
			"📅 <b>Шаг 3/5</b>\n\n"+
				"Введите дату вылета (ГГГГ-ММ-ДД) или напишите 'любая':",
		)

	case phaseDateFrom:
		data.DateFrom = text
		b.phaseMap[userID] = phaseNights
		return c.Send(
			"🌙 <b>Шаг 4/5</b>\n\n"+
				"Введите количество ночей (например: 7, 10, 14):",
		)

	case phaseNights:
		nights, err := strconv.Atoi(text)
		if err != nil || nights < 1 || nights > 60 {
			return c.Send("⚠️ Введите корректное количество ночей (от 1 до 60):")
		}
		data.Nights = text
		b.phaseMap[userID] = phaseMaxPrice
		return c.Send(
			"💰 <b>Шаг 5/5</b>\n\n"+
				"Введите максимальный бюджет на человека (руб) или напишите 'любой':",
		)

	case phaseMaxPrice:
		delete(b.phaseMap, userID)
		return b.createSubscription(c, userID, text)

	default:
		delete(b.phaseMap, userID)
		return nil
	}
}

func (b *Bot) createSubscription(c tb.Context, userID int64, maxPriceText string) error {
	data := b.subData[userID]
	delete(b.subData, userID)

	// Ensure user exists
	ctx := context.Background()
	user, err := b.storage.EnsureUser(ctx, userID, "")
	if err != nil {
		b.logger.Printf("EnsureUser error: %v", err)
		return c.Send("❌ Ошибка при регистрации. Попробуйте позже.")
	}

	// Create subscription (simplified - no date validation for MVP)
	sub := &domain.Subscription{
		UserID:       user.ID,
		OriginCity:   data.OriginCity,
		Destination:  data.Destination,
		NightsMin:    7,
		NightsMax:    14,
		MaxPrice:     0, // Will parse in production
		IsActive:     true,
		Messenger:    "telegram",
		UserIdentifier: fmt.Sprintf("%d", userID),
	}

	if maxPriceText != "любой" && maxPriceText != "" {
		if price, err := strconv.Atoi(maxPriceText); err == nil {
			sub.MaxPrice = price
		}
	}

	if data.Nights != "" {
		if nights, err := strconv.Atoi(data.Nights); err == nil {
			sub.NightsMin = nights
			sub.NightsMax = nights
		}
	}

	err = b.storage.CreateSubscription(ctx, sub)
	if err != nil {
		b.logger.Printf("CreateSubscription error: %v", err)
		return c.Send("❌ Ошибка при создании подписки. Попробуйте позже.")
	}

	priceFilter := ""
	if sub.MaxPrice > 0 {
		priceFilter = fmt.Sprintf(" | до %d руб", sub.MaxPrice)
	}

	return c.Send(
		fmt.Sprintf(
			"✅ <b>Подписка добавлена!</b>\n\n"+
				"🛫 %s → %s\n"+
				"🌙 %d ночей%s\n\n"+
				"Я уведомлю вас, когда найду выгодную цену!",
			sub.OriginCity, sub.Destination, sub.NightsMin, priceFilter,
		),
		&tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{
				{
					{Text: "📋 Мои подписки", Data: "subs_list"},
					{Text: "➕ Ещё", Data: "add_start"},
				},
			},
		},
	)
}

func (b *Bot) handleListSubs(c tb.Context) error {
	ctx := context.Background()
	user, err := b.storage.GetUserByTGChatID(ctx, c.Sender().ID)
	if err != nil {
		return c.Send("❌ Ошибка при поиске пользователя.")
	}
	if user == nil {
		return c.Send("❌ Вы не зарегистрированы. Отправьте /start.")
	}

	subs, err := b.storage.GetUserActiveSubscriptions(ctx, user.ID)
	if err != nil {
		return c.Send("❌ Ошибка при загрузке подписок.")
	}

	if len(subs) == 0 {
		return c.Send(
			"📭 <b>У вас нет активных подписок</b>\n\n"+
				"Добавьте первое направление:",
			&tb.ReplyMarkup{
				InlineKeyboard: [][]tb.InlineButton{
					{
						{Text: "➕ Добавить подписку", Data: "add_start"},
					},
				},
			},
		)
	}

	msg := "<b>📋 Ваши подписки:</b>\n\n"
	for i, sub := range subs {
		msg += fmt.Sprintf("<b>%d.</b> %s → %s", i+1, sub.OriginCity, sub.Destination)
		if sub.MaxPrice > 0 {
			msg += fmt.Sprintf(" | до %d руб", sub.MaxPrice)
		}
		msg += fmt.Sprintf(" | %d-%d ночей\n", sub.NightsMin, sub.NightsMax)
	}

	msg += "\nУдалить: /remove <номер>"

	markup := tb.ReplyMarkup{
		InlineKeyboard: [][]tb.InlineButton{
			{
				{Text: "➕ Добавить ещё", Data: "add_start"},
				{Text: "🔄 Обновить", Data: "subs_list"},
			},
		},
	}

	return c.Send(msg, &markup)
}

func (b *Bot) handleRemoveSub(c tb.Context) error {
	ctx := context.Background()
	user, err := b.storage.GetUserByTGChatID(ctx, c.Sender().ID)
	if err != nil || user == nil {
		return c.Send("❌ Пользователь не найден.")
	}

	// Parse subscription ID from command
	args := strings.Fields(c.Text())
	if len(args) < 2 {
		return c.Send(
			"❓ Укажите номер подписки для удаления:\n\n"+
				"📋 /subs — посмотреть список\n"+
				"❌ /remove 1 — удалить первую подписку",
		)
	}

	subID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return c.Send("⚠️ Неверный номер подписки. Введите число.")
	}

	// Verify ownership
	subs, err := b.storage.GetUserActiveSubscriptions(ctx, user.ID)
	if err != nil {
		return c.Send("❌ Ошибка.")
	}

	found := false
	for _, sub := range subs {
		if sub.ID == subID {
			_ = b.storage.DeactivateSubscription(ctx, subID)
			found = true
			return c.Send(
				fmt.Sprintf(
					"✅ Подписка удалена!\n\n"+
						"🛫 %s → %s",
					sub.OriginCity, sub.Destination,
				),
				&tb.ReplyMarkup{
					InlineKeyboard: [][]tb.InlineButton{
						{
							{Text: "📋 Мои подписки", Data: "subs_list"},
							{Text: "➕ Добавить", Data: "add_start"},
						},
					},
				},
			)
		}
	}

	if found {
		return c.Send("✅ Подписка удалена.")
	}
	return c.Send("❌ Подписка не найдена. Используйте /subs для просмотра.")
}

// SendTourNotification sends a tour notification to a user
func (b *Bot) SendTourNotification(ctx context.Context, userID int64, offer *domain.TourOffer, sub *domain.Subscription, reason string) error {
	var reasonText string
	var reasonEmoji string
	switch reason {
	case "new":
		reasonText = "Новая находка!"
		reasonEmoji = "🔥"
	case "price_drop":
		reasonText = "Цена упала!"
		reasonEmoji = "📉"
	default:
		reasonText = "Выгодный тур!"
		reasonEmoji = "✨"
	}

	countryEmoji := "🌍"
	destLower := strings.ToLower(sub.Destination)
	if strings.Contains(destLower, "турция") {
		countryEmoji = "🇹🇷"
	} else if strings.Contains(destLower, "египет") {
		countryEmoji = "🇪🇬"
	} else if strings.Contains(destLower, "оаэ") || strings.Contains(destLower, "дубай") {
		countryEmoji = "🇦🇪"
	} else if strings.Contains(destLower, "тайланд") || strings.Contains(destLower, "паттайя") || strings.Contains(destLower, "пукет") {
		countryEmoji = "🇹🇭"
	} else if strings.Contains(destLower, "сочи") || strings.Contains(destLower, "абхазия") {
		countryEmoji = "🇷🇺"
	}

	priceText := fmt.Sprintf("%d ₽", offer.Price)
	if sub.MaxPrice > 0 {
		priceText = fmt.Sprintf("%d ₽ < %d ₽", offer.Price, sub.MaxPrice)
	}

	msg := fmt.Sprintf(
		"%s %s\n\n"+
			"<b>%s %s</b>\n"+
			"🛫 %s → %s\n\n"+
			"🏨 <b>%s</b>\n"+
			"⭐ %d звёзд\n"+
			"🌙 %d ночей\n"+
			"💰 <b>%s</b>",
		reasonEmoji, reasonText,
		countryEmoji, sub.Destination,
		sub.OriginCity, sub.Destination,
		offer.HotelName,
		offer.HotelStars,
		offer.Nights,
		priceText,
	)

	markup := &tb.ReplyMarkup{
		InlineKeyboard: [][]tb.InlineButton{
			{{Text: "👉 Забронировать", URL: offer.AffiliateURL}},
		},
	}

	_, err := b.teleBot.Send(&tb.User{ID: userID}, msg, markup)
	return err
}

// Start begins the Telegram poller
func (b *Bot) Start() {
	b.logger.Println("Telegram bot starting...")
	b.teleBot.Start()
}

// Stop stops the Telegram poller
func (b *Bot) Stop() {
	b.teleBot.Stop()
}
