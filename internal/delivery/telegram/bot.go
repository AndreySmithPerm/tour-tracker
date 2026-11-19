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
		return c.Send("Привет! Я — сервис отслеживания лучших цен на туры.\n\n"+
			"Добавьте направление, и я уведомлю вас, когда цена упадёт.\n\n"+
			"Используйте /add чтобы начать.")
	})

	b.teleBot.Handle("/help", func(c tb.Context) error {
		return c.Send("/add — добавить подписку на отслеживание туров\n" +
			"/subs — список активных подписок\n" +
			"/remove <id> — удалить подписку\n" +
			"/help — справка")
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
		return c.Send("Отмена.")
	})
}

func (b *Bot) handleAddStart(c tb.Context) error {
	userID := c.Sender().ID
	b.phaseMap[userID] = phaseOriginCity
	b.subData[userID] = &subscriptionData{}
	return c.Send("Введите город вылета (например: Москва):")
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
		data.OriginCity = text
		b.phaseMap[userID] = phaseDestination
		return c.Send("Введите направление (страну/город):")

	case phaseDestination:
		data.Destination = text
		b.phaseMap[userID] = phaseDateFrom
		return c.Send("Введите дату вылета (ГГГГ-ММ-ДД) или напишите 'любая':")

	case phaseDateFrom:
		data.DateFrom = text
		b.phaseMap[userID] = phaseNights
		return c.Send("Введите количество ночей (например: 7):")

	case phaseNights:
		data.Nights = text
		b.phaseMap[userID] = phaseMaxPrice
		return c.Send("Введите максимальный бюджет на человека (руб) или напишите 'любой':")

	case phaseMaxPrice:
		// All info collected
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
		return c.Send("Ошибка при регистрации. Попробуйте позже.")
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
		return c.Send("Ошибка при создании подписки. Попробуйте позже.")
	}

	priceFilter := ""
	if sub.MaxPrice > 0 {
		priceFilter = fmt.Sprintf(" (до %d руб)", sub.MaxPrice)
	}

	return c.Send(fmt.Sprintf(
		"✅ Подписка добавлена!\n\n"+
			"%s → %s\n"+
			"Ночей: %d%s\n\n"+
			"Я уведомлю вас, когда найду выгодную цену.",
		sub.OriginCity, sub.Destination, sub.NightsMin, priceFilter,
	))
}

func (b *Bot) handleListSubs(c tb.Context) error {
	ctx := context.Background()
	user, err := b.storage.GetUserByTGChatID(ctx, c.Sender().ID)
	if err != nil {
		return c.Send("Ошибка при поиске пользователя.")
	}
	if user == nil {
		return c.Send("Вы не зарегистрированы. Отправьте /start.")
	}

	subs, err := b.storage.GetUserActiveSubscriptions(ctx, user.ID)
	if err != nil {
		return c.Send("Ошибка при загрузке подписок.")
	}

	if len(subs) == 0 {
		return c.Send("У вас нет активных подписок. Используйте /add для добавления.")
	}

	msg := "Ваши подписки:\n\n"
	for i, sub := range subs {
		msg += fmt.Sprintf("%d. %s → %s", i+1, sub.OriginCity, sub.Destination)
		if sub.MaxPrice > 0 {
			msg += fmt.Sprintf(" | до %d руб", sub.MaxPrice)
		}
		msg += fmt.Sprintf(" | %d-%d ночей\n", sub.NightsMin, sub.NightsMax)
	}

	msg += "\nУдалить: /remove <номер>"
	return c.Send(msg)
}

func (b *Bot) handleRemoveSub(c tb.Context) error {
	ctx := context.Background()
	user, err := b.storage.GetUserByTGChatID(ctx, c.Sender().ID)
	if err != nil || user == nil {
		return c.Send("Пользователь не найден.")
	}

	// Parse subscription ID from command
	args := strings.Fields(c.Text())
	if len(args) < 2 {
		return c.Send("Укажите номер подписки: /remove 1")
	}

	subID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return c.Send("Неверный номер подписки.")
	}

	// Verify ownership
	subs, err := b.storage.GetUserActiveSubscriptions(ctx, user.ID)
	if err != nil {
		return c.Send("Ошибка.")
	}

	found := false
	for _, sub := range subs {
		if sub.ID == subID {
			_ = b.storage.DeactivateSubscription(ctx, subID)
			found = true
			break
		}
	}

	if found {
		return c.Send("Подписка удалена.")
	}
	return c.Send("Подписка не найдена.")
}

// SendTourNotification sends a tour notification to a user
func (b *Bot) SendTourNotification(ctx context.Context, userID int64, offer *domain.TourOffer, sub *domain.Subscription, reason string) error {
	var reasonText string
	switch reason {
	case "new":
		reasonText = "🔥 Новая находка!"
	case "price_drop":
		reasonText = "📉 Цена упала!"
	default:
		reasonText = "✨ Выгодный тур!"
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
		"%s %s %s → %s\n\n"+
			"🏨 %s\n"+
			"⭐ %d звёзд\n"+
			"🌙 %d ночей\n"+
			"💰 %s\n\n"+
			"👉 %s",
		reasonText,
		countryEmoji,
		sub.OriginCity, sub.Destination,
		offer.HotelName,
		offer.HotelStars,
		offer.Nights,
		priceText,
		offer.AffiliateURL,
	)

	_, err := b.teleBot.Send(&tb.User{ID: userID}, msg)
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
