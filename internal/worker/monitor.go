package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/travel-to-tour/tour-tracker/internal/affiliate"
	delivery "github.com/travel-to-tour/tour-tracker/internal/delivery"
	"github.com/travel-to-tour/tour-tracker/internal/delivery/telegram"
	"github.com/travel-to-tour/tour-tracker/internal/domain"
	"github.com/travel-to-tour/tour-tracker/internal/provider"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// Monitor groups subscriptions, checks prices, and sends notifications
type Monitor struct {
	storage     *storage.Storage
	providers   []provider.TourProvider
	affiliate   *affiliate.Client
	telegram    *telegram.Bot
	max         *delivery.MaxBot
	logger      *log.Logger
	stopCh      chan struct{}
}

// NewMonitor creates a new price monitor
func NewMonitor(
	s *storage.Storage,
	providers []provider.TourProvider,
	aff *affiliate.Client,
	tg *telegram.Bot,
	max *delivery.MaxBot,
) *Monitor {
	return &Monitor{
		storage:   s,
		providers: providers,
		affiliate: aff,
		telegram:  tg,
		max:       max,
		logger:    log.Default(),
		stopCh:    make(chan struct{}),
	}
}

// Run starts the monitoring loop
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	m.logger.Println("Monitor started")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	m.runCheck(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Println("Monitor stopping: context cancelled")
			return
		case <-m.stopCh:
			m.logger.Println("Monitor stopping: stopped explicitly")
			return
		case <-ticker.C:
			m.runCheck(ctx)
		}
	}
}

// Stop gracefully stops the monitor
func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) runCheck(ctx context.Context) {
	m.logger.Println("Running price check cycle...")

	// 1. Fetch all active subscriptions
	subs, err := m.storage.GetActiveSubscriptions(ctx)
	if err != nil {
		m.logger.Printf("Error fetching subscriptions: %v", err)
		return
	}

	if len(subs) == 0 {
		m.logger.Println("No active subscriptions, skipping")
		return
	}

	// 2. Group subscriptions by search key (deduplication)
	grouped := m.groupSubscriptions(subs)
	m.logger.Printf("Found %d unique search keys from %d subscriptions", len(grouped), len(subs))

	// 3. For each unique search key, query providers
	for searchKey, userSubs := range grouped {
		m.checkSearchKey(ctx, searchKey, userSubs)
	}

	m.logger.Println("Price check cycle completed")
}

// groupSubscriptions groups active subscriptions by search key for deduplication
func (m *Monitor) groupSubscriptions(subs []*domain.Subscription) map[domain.SearchKey][]*domain.Subscription {
	grouped := make(map[domain.SearchKey][]*domain.Subscription)

	for _, sub := range subs {
		key := domain.SearchKey{
			OriginCity:  strings.ToLower(sub.OriginCity),
			Destination: strings.ToLower(sub.Destination),
			Country:     strings.ToLower(sub.Country),
			MonthBucket: sub.DateFrom.Format("2006-01"),
		}

		grouped[key] = append(grouped[key], sub)
	}

	return grouped
}

func (m *Monitor) checkSearchKey(ctx context.Context, key domain.SearchKey, subs []*domain.Subscription) {
	searchHash := storage.GenerateSearchHash(key.OriginCity, key.Destination, key.Country, key.MonthBucket)

	// Query all providers for this search key
	var allOffers []*domain.TourOffer
	for _, provider := range m.providers {
		offers, err := provider.Search(ctx, key)
		if err != nil {
			m.logger.Printf("Provider %s error for %s: %v", provider.Name(), searchHash, err)
			continue
		}
		allOffers = append(allOffers, offers...)
	}

	if len(allOffers) == 0 {
		return
	}

	// Find the minimum price per hotel
	hotelMinPrice := make(map[string]*domain.TourOffer)
	for _, offer := range allOffers {
		hash := storage.GenerateTourHash(offer.HotelName, offer.Nights, offer.Price, offer.Provider)
		if existing, ok := hotelMinPrice[hash]; ok {
			if offer.Price < existing.Price {
				hotelMinPrice[hash] = offer
			}
		} else {
			hotelMinPrice[hash] = offer
		}
	}

	// Generate affiliate links for all tours
	tourURLs := make([]affiliate.LinkItem, 0, len(hotelMinPrice))
	for _, offer := range hotelMinPrice {
		tourURLs = append(tourURLs, affiliate.LinkItem{
			URL:   offer.TourURL,
			SubID: "worker",
		})
	}

	affiliateURLs, err := m.affiliate.GeneratePartnerLinks(ctx, tourURLs)
	if err != nil {
		m.logger.Printf("Affiliate link generation error: %v", err)
	}

	// Compare with snapshots and notify
	for hash, offer := range hotelMinPrice {
		if affiliateURL, ok := affiliateURLs[offer.TourURL]; ok {
			offer.AffiliateURL = affiliateURL
		}

		// Save offer to DB
		offer.SearchHash = searchHash
		_ = m.storage.CreateTourOffer(ctx, offer)

		// Check against snapshot
		snap, err := m.storage.GetSnapshot(ctx, searchHash)
		if err != nil && err != pg.ErrNoRows {
			m.logger.Printf("Snapshot fetch error for %s: %v", searchHash, err)
			continue
		}

		shouldNotify := false
		var notifyReason string

		if snap == nil {
			// First time seeing this search key
			shouldNotify = true
			notifyReason = "new"
		} else if offer.Price < snap.MinPrice {
			// Price dropped
			dropRatio := float64(snap.MinPrice-offer.Price) / float64(snap.MinPrice)
			if dropRatio >= 0.10 { // At least 10% drop
				shouldNotify = true
				notifyReason = fmt.Sprintf("price_drop_%.0f", dropRatio*100)
			}
		}

		if shouldNotify {
			// Update snapshot
			newSnap := &domain.PriceSnapshot{
				SearchHash:   searchHash,
				MinPrice:     offer.Price,
				TourURL:      offer.TourURL,
				AffiliateURL: offer.AffiliateURL,
				UpdatedAt:    time.Now(),
			}
			_ = m.storage.UpsertSnapshot(ctx, newSnap)

			// Send notifications to matching users
			m.notifyUsers(ctx, subs, offer, hash, notifyReason)
		}
	}
}

func (m *Monitor) notifyUsers(ctx context.Context, subs []*domain.Subscription, offer *domain.TourOffer, tourHash, reason string) {
	// Group subscriptions by user
	userSubs := make(map[int64]*domain.Subscription)
	for _, sub := range subs {
		userSubs[sub.UserID] = sub
	}

	for userID, sub := range userSubs {
		// Check if notification was already sent
		sent, err := m.storage.IsNotificationSent(ctx, userID, tourHash)
		if err != nil {
			m.logger.Printf("Notification check error for user %d: %v", userID, err)
			continue
		}
		if sent {
			continue
		}

		// Check price threshold
		if sub.MaxPrice > 0 && offer.Price > sub.MaxPrice {
			continue
		}

		// Check nights range
		if offer.Nights < sub.NightsMin || offer.Nights > sub.NightsMax {
			continue
		}

		// Send notification via selected messenger
		if sub.Messenger == "telegram" && m.telegram != nil {
			err = m.telegram.SendTourNotification(ctx, userID, offer, sub, reason)
			if err != nil {
				m.logger.Printf("Telegram send error for user %d: %v", userID, err)
				continue
			}
		} else if sub.Messenger == "max" && m.max != nil {
			err = m.max.SendTourNotification(ctx, sub.UserIdentifier, offer, sub, reason)
			if err != nil {
				m.logger.Printf("MAX send error for user %s: %v", sub.UserIdentifier, err)
				continue
			}
		} else {
			m.logger.Printf("No messenger bot configured for %s — skipping notification for user %d", sub.Messenger, userID)
			continue
		}

		// Record notification
		notif := &domain.SentNotification{
			UserID:   userID,
			TourHash: tourHash,
			SentAt:   time.Now(),
		}
		_ = m.storage.RecordNotification(ctx, notif)

		m.logger.Printf("Notification sent to user %d for tour %s", userID, offer.HotelName)
	}
}
