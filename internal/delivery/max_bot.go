package delivery

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/travel-to-tour/tour-tracker/internal/domain"
)

// MaxBot handles notifications via MAX messenger
type MaxBot struct {
	apiKey   string
	baseURL  string
	logger   *log.Logger
}

// NewMaxBot creates a new MAX bot instance
func NewMaxBot(apiKey, baseURL string, logger *log.Logger) *MaxBot {
	if baseURL == "" {
		baseURL = "https://api.max.ru" // placeholder
	}
	return &MaxBot{
		apiKey:  apiKey,
		baseURL: baseURL,
		logger:  logger,
	}
}

// SendTourNotification sends a tour notification to a user via MAX
func (m *MaxBot) SendTourNotification(ctx context.Context, userID string, offer *domain.TourOffer, sub *domain.Subscription, reason string) error {
	m.logger.Printf("MAX: Sending notification to user %s for tour %s", userID, offer.HotelName)

	// TODO: Implement actual MAX API call
	// POST {baseURL}/messages
	// Body: {user_id: userID, message: messageText, api_key: m.apiKey}

	return nil
}

// BuildNotificationMessage creates a notification message for MAX
func BuildMaxNotificationMessage(offer *domain.TourOffer, sub *domain.Subscription, reason string) string {
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

	return fmt.Sprintf(
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
}
