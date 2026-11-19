package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-pg/pg/v10"
	"github.com/travel-to-tour/tour-tracker/internal/domain"
)

type Storage struct {
	db *pg.DB
}

// NewFromURL creates storage from a full DATABASE_URL
func NewFromURL(databaseURL string) (*Storage, error) {
	opts, err := parseDSN(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	db := pg.Connect(opts)

	// Test connection
	_, err = db.Exec(context.Background(), "SELECT 1")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Storage{db: db}, nil
}

// parseDSN parses a PostgreSQL DSN into pg.Options
func parseDSN(dsn string) (*pg.Options, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return pg.ParseURL(dsn)
	}
	// Assume it's a DSN string
	return pg.ParseURL(dsn)
}

// DB returns the underlying pg.DB for raw queries
func (s *Storage) DB() *pg.DB {
	return s.db
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// --- User operations ---

func (s *Storage) CreateUser(ctx context.Context, user *domain.User) error {
	_, err := s.db.ModelContext(ctx, user).Insert()
	return err
}

func (s *Storage) GetUserByTGChatID(ctx context.Context, chatID int64) (*domain.User, error) {
	user := &domain.User{}
	err := s.db.ModelContext(ctx, user).
		Where("tg_chat_id = ?", chatID).
		First()
	if err != nil {
		if err == pg.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *Storage) EnsureUser(ctx context.Context, chatID int64, username string) (*domain.User, error) {
	user, err := s.GetUserByTGChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	user = &domain.User{
		TGChatID:   chatID,
		TGUsername: username,
	}
	if err := s.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// --- Subscription operations ---

func (s *Storage) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	_, err := s.db.ModelContext(ctx, sub).Insert()
	return err
}

func (s *Storage) GetUserActiveSubscriptions(ctx context.Context, userID int64) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := s.db.ModelContext(ctx, &subs).
		Where("user_id = ?", userID).
		Where("is_active = true").
		Select()
	return subs, err
}

func (s *Storage) GetActiveSubscriptions(ctx context.Context) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := s.db.ModelContext(ctx, &subs).
		Where("is_active = true").
		Select()
	return subs, err
}

func (s *Storage) DeactivateSubscription(ctx context.Context, subID int64) error {
	_, err := s.db.ModelContext(ctx, &domain.Subscription{}).
		Where("id = ?", subID).
		Set("is_active = false").
		Update()
	return err
}

// --- PriceSnapshot operations ---

func (s *Storage) UpsertSnapshot(ctx context.Context, snap *domain.PriceSnapshot) error {
	_, err := s.db.ModelContext(ctx, snap).
		OnConflict("(search_hash) DO UPDATE").
		Set("min_price = EXCLUDED.min_price").
		Set("tour_url = EXCLUDED.tour_url").
		Set("affiliate_url = EXCLUDED.affiliate_url").
		Set("updated_at = NOW()").
		Returning("").
		Exec(nil)
	return err
}

func (s *Storage) GetSnapshot(ctx context.Context, searchHash string) (*domain.PriceSnapshot, error) {
	snap := &domain.PriceSnapshot{}
	err := s.db.ModelContext(ctx, snap).
		Where("search_hash = ?", searchHash).
		First()
	if err != nil {
		if err == pg.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return snap, nil
}

// --- TourOffer operations ---

func (s *Storage) CreateTourOffer(ctx context.Context, offer *domain.TourOffer) error {
	_, err := s.db.ModelContext(ctx, offer).Insert()
	return err
}

func (s *Storage) SaveTourOffers(ctx context.Context, offers []*domain.TourOffer) error {
	if len(offers) == 0 {
		return nil
	}
	_, err := s.db.ModelContext(ctx, &offers).Insert()
	return err
}

// --- SentNotification operations ---

func (s *Storage) IsNotificationSent(ctx context.Context, userID int64, tourHash string) (bool, error) {
	count, err := s.db.ModelContext(ctx, &domain.SentNotification{}).
		Where("user_id = ?", userID).
		Where("tour_hash = ?", tourHash).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Storage) RecordNotification(ctx context.Context, notif *domain.SentNotification) error {
	_, err := s.db.ModelContext(ctx, notif).Insert()
	return err
}

// --- IP Registration tracking (anti-spam) ---

func (s *Storage) RecordIPRegistration(ctx context.Context, ip string, userID *int64) error {
	reg := &domain.IPRegistration{
		IP:     ip,
		UserID: userID,
	}
	_, err := s.db.ModelContext(ctx, reg).Insert()
	return err
}

func (s *Storage) CountRegistrationsTodayByIP(ctx context.Context, ip string) (int, error) {
	count, err := s.db.ModelContext(ctx, &domain.IPRegistration{}).
		Where("ip_address = ?", ip).
		Where("created_at >= NOW() - INTERVAL '24 hours'").
		Count()
	return count, err
}

// --- IP Rate Limit tracking ---

func (s *Storage) RecordRateLimit(ctx context.Context, ip, action string) error {
	rl := &domain.IPRateLimit{
		IP:     ip,
		Action: action,
	}
	_, err := s.db.ModelContext(ctx, rl).Insert()
	return err
}

func (s *Storage) CountActionsTodayByIP(ctx context.Context, ip, action string) (int, error) {
	count, err := s.db.ModelContext(ctx, &domain.IPRateLimit{}).
		Where("ip_address = ?", ip).
		Where("action = ?", action).
		Where("created_at >= NOW() - INTERVAL '24 hours'").
		Count()
	return count, err
}

// --- Client Requests tracking ---

func (s *Storage) RecordClientRequest(ctx context.Context, ip, path, method string, userID *int64) error {
	req := &domain.ClientRequest{
		IP:     ip,
		Path:   path,
		Method: method,
		UserID: userID,
	}
	_, err := s.db.ModelContext(ctx, req).Insert()
	return err
}

// CountRequestsTodayByIP returns number of requests from an IP today
func (s *Storage) CountRequestsTodayByIP(ctx context.Context, ip string) (int, error) {
	count, err := s.db.ModelContext(ctx, &domain.ClientRequest{}).
		Where("ip_address = ?", ip).
		Where("created_at >= NOW() - INTERVAL '24 hours'").
		Count()
	return count, err
}

// --- SearchKey helpers ---

func GenerateSearchHash(originCity, destination, country, monthBucket string) string {
	return fmt.Sprintf("%s|%s|%s|%s", originCity, destination, country, monthBucket)
}

func GenerateTourHash(hotelName string, nights int, price int, provider string) string {
	return fmt.Sprintf("%s|%d|%d|%s", hotelName, nights, price, provider)
}
