package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/travel-to-tour/tour-tracker/internal/domain"
)

// MaxBot handles notifications via MAX messenger
type MaxBot struct {
	apiKey  string
	baseURL string
	logger  *log.Logger
	httpClient *http.Client
}

// MaxSendMessageRequest is the request body for POST /messages
type MaxSendMessageRequest struct {
	Text        string              `json:"text"`
	Attachments []MaxAttachment     `json:"attachments,omitempty"`
	Format      string              `json:"format,omitempty"`
}

// MaxAttachment represents a message attachment
type MaxAttachment struct {
	Type    string              `json:"type"`
	Payload MaxAttachmentPayload `json:"payload"`
}

// MaxAttachmentPayload for inline_keyboard
type MaxAttachmentPayload struct {
	Buttons [][]MaxButton `json:"buttons"`
}

// MaxButton represents a button in an inline keyboard
type MaxButton struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	URL     string `json:"url,omitempty"`
	Payload string `json:"payload,omitempty"`
}

// MaxSendMessageResponse is the response from POST /messages
type MaxSendMessageResponse struct {
	Success   bool   `json:"success,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// NewMaxBot creates a new MAX bot instance
func NewMaxBot(apiKey, baseURL string, logger *log.Logger) *MaxBot {
	if baseURL == "" {
		baseURL = "https://platform-api2.max.ru"
	}
	return &MaxBot{
		apiKey:     apiKey,
		baseURL:    baseURL,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendTourNotification sends a tour notification to a user via MAX
func (m *MaxBot) SendTourNotification(ctx context.Context, userID string, offer *domain.TourOffer, sub *domain.Subscription, reason string) error {
	// Parse user_id from identifier (it should be a numeric string)
	userIDInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		// Try to extract numeric part from @username format
		userIDStr := strings.TrimPrefix(userID, "@")
		userIDInt, err = strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user identifier %q: %w", userID, err)
		}
	}

	messageText := BuildMaxNotificationMessage(offer, sub, reason)

	// Build inline keyboard with booking link
	req := MaxSendMessageRequest{
		Text:   messageText,
		Format: "markdown",
		Attachments: []MaxAttachment{
			{
				Type: "inline_keyboard",
				Payload: MaxAttachmentPayload{
					Buttons: [][]MaxButton{
						{
							{
								Type: "link",
								Text: "Забронировать",
								URL:  offer.AffiliateURL,
							},
						},
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/messages?user_id=%d", m.baseURL, userIDInt)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", m.apiKey)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request to MAX failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	m.logger.Printf("MAX API response: status=%d body=%s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MAX API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Check for success field in response
	var apiResp MaxSendMessageResponse
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if !apiResp.Success && apiResp.Code != "" {
			return fmt.Errorf("MAX API error: %s - %s", apiResp.Code, apiResp.Message)
		}
	}

	m.logger.Printf("MAX: notification sent to user %d for tour %s", userIDInt, offer.HotelName)
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
			"🏨 *%s*\n"+
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
