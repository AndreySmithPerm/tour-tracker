package domain

import (
	"time"
)

// User represents a Telegram user or website visitor
type User struct {
	ID        int64     `pg:"id,pk,autoincrement"`
	TGChatID  int64     `pg:"tg_chat_id,notnull"`
	TGUsername string   `pg:"tg_username"`
	CreatedAt time.Time `pg:"created_at,notnull,default:now()"`
}

// Subscription represents a user's tour search preference
type Subscription struct {
	ID            int64     `pg:"id,pk,autoincrement"`
	UserID        int64     `pg:"user_id,notnull"`
	OriginCity    string    `pg:"origin_city,notnull"`
	Destination   string    `pg:"destination,notnull"`
	Country       string    `pg:"country"`
	DateFrom      time.Time `pg:"date_from"`
	DateTo        time.Time `pg:"date_to"`
	NightsMin     int       `pg:"nights_min,default:7"`
	NightsMax     int       `pg:"nights_max,default:14"`
	MaxPrice      int       `pg:"max_price"`          // maximum price in rubles
	MinPriceDrop  float64   `pg:"min_price_drop,default:0.10"` // minimum price drop ratio to trigger notification (10%)
	IsActive      bool      `pg:"is_active,default:true"`
	CreatedAt     time.Time `pg:"created_at,notnull,default:now()"`
}

// SearchKey is a deduplication key for grouping subscriptions
type SearchKey struct {
	OriginCity string
	Destination string
	Country    string
	MonthBucket string // e.g. "2026-07" for July 2026
}

// TourOffer represents a tour found from a provider
type TourOffer struct {
	ID         int64     `pg:"id,pk,autoincrement"`
	SearchHash string    `pg:"search_hash,notnull,unique"`
	OriginCity string    `pg:"origin_city,notnull"`
	Destination string   `pg:"destination,notnull"`
	HotelName  string    `pg:"hotel_name"`
	HotelStars int       `pg:"hotel_stars"`
	Nights     int       `pg:"nights"`
	Price      int       `pg:"price,notnull"`        // price in rubles
	Provider   string    `pg:"provider,notnull"`     // e.g. "travelata", "level"
	TourURL    string    `pg:"tour_url,notnull"`     // direct URL to the tour
	AffiliateURL string  `pg:"affiliate_url"`        // partner URL from Travelpayouts
	UpdatedAt  time.Time `pg:"updated_at,notnull,default:now()"`
}

// PriceSnapshot stores the last known minimum price for a search key
type PriceSnapshot struct {
	SearchHash  string    `pg:"search_hash,notnull,unique"`
	MinPrice    int       `pg:"min_price,notnull"`
	TourURL     string    `pg:"tour_url"`
	AffiliateURL string   `pg:"affiliate_url"`
	UpdatedAt   time.Time `pg:"updated_at,notnull,default:now()"`
}

// SentNotification prevents duplicate notifications
type SentNotification struct {
	ID         int64     `pg:"id,pk,autoincrement"`
	UserID     int64     `pg:"user_id,notnull"`
	TourHash   string    `pg:"tour_hash,notnull"`
	SentAt     time.Time `pg:"sent_at,notnull,default:now()"`
}

// IPRegistration tracks registrations by IP for anti-spam
type IPRegistration struct {
	ID       int64     `pg:"id,pk,autoincrement"`
	IP       string    `pg:"ip_address,notnull"`
	UserID   *int64    `pg:"user_id"`
	CreatedAt time.Time `pg:"created_at,notnull,default:now()"`
}

// IPRateLimit tracks rate limits per IP
type IPRateLimit struct {
	ID       int64     `pg:"id,pk,autoincrement"`
	IP       string    `pg:"ip_address,notnull"`
	Action   string    `pg:"action,notnull"` // 'subscribe', 'link_generate'
	CreatedAt time.Time `pg:"created_at,notnull,default:now()"`
}

// ClientRequest tracks all client requests for daily limit enforcement
type ClientRequest struct {
	ID       int64     `pg:"id,pk,autoincrement"`
	IP       string    `pg:"ip_address,notnull"`
	Path     string    `pg:"path,notnull"`
	Method   string    `pg:"method,notnull"`
	UserID   *int64    `pg:"user_id"`
	CreatedAt time.Time `pg:"created_at,notnull,default:now()"`
}
