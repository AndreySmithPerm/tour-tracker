package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	sec "github.com/travel-to-tour/tour-tracker/internal/middleware"
	"github.com/travel-to-tour/tour-tracker/internal/domain"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

type API struct {
	storage *storage.Storage
	cfg     sec.Config
	jwtCfg  sec.JWTConfig
}

func NewAPI(s *storage.Storage, cfg sec.Config, jwtCfg sec.JWTConfig) *API {
	return &API{
		storage: s,
		cfg:     cfg,
		jwtCfg:  jwtCfg,
	}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(sec.ClientRequestMiddleware(a.storage, a.cfg))

	r.Get("/health", a.handleHealth)
	r.Get("/tours/hot", a.handleHotTours)

	r.Post("/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		next := http.HandlerFunc(a.handleCreateSubscription)
		limited := sec.SubscriptionLimitMiddleware(a.storage, a.cfg)(next)
		limited.ServeHTTP(w, r)
	})

	r.Group(func(r chi.Router) {
		r.Use(sec.AuthRequiredMiddleware(a.jwtCfg))
		r.Get("/me/subscriptions", a.handleMySubscriptions)
		r.Delete("/me/subscriptions/{id}", a.handleDeleteSubscription)
		r.Get("/me", a.handleMe)
	})

	return r
}
type hotTourResponse struct {
	ID           int64     `json:"id"`
	SearchHash   string    `json:"search_hash"`
	OriginCity   string    `json:"origin_city"`
	Destination  string    `json:"destination"`
	HotelName    string    `json:"hotel_name"`
	HotelStars   int       `json:"hotel_stars"`
	Nights       int       `json:"nights"`
	Price        int       `json:"price"`
	Provider     string    `json:"provider"`
	TourURL      string    `json:"tour_url"`
	AffiliateURL string    `json:"affiliate_url"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleHotTours(w http.ResponseWriter, r *http.Request) {
	type rawTour struct {
		ID           int64     `pg:"id"`
		SearchHash   string    `pg:"search_hash"`
		OriginCity   string    `pg:"origin_city"`
		Destination  string    `pg:"destination"`
		HotelName    string    `pg:"hotel_name"`
		HotelStars   int       `pg:"hotel_stars"`
		Nights       int       `pg:"nights"`
		Price        int       `pg:"price"`
		Provider     string    `pg:"provider"`
		TourURL      string    `pg:"tour_url"`
		AffiliateURL string    `pg:"affiliate_url"`
		UpdatedAt    time.Time `pg:"updated_at"`
	}

	var results []rawTour
	err := a.storage.DB().ModelContext(r.Context(), &results).
		TableExpr("(SELECT DISTINCT ON (search_hash, hotel_name) id, search_hash, origin_city, destination, hotel_name, hotel_stars, nights, price, provider, tour_url, affiliate_url, updated_at FROM tour_offers ORDER BY search_hash, hotel_name, price ASC LIMIT 100)").
		Select()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tours":   []hotTourResponse{},
			"message": "Пока нет предложений",
		})
		return
	}

	tours := make([]hotTourResponse, 0, len(results))
	for _, t := range results {
		tours = append(tours, hotTourResponse{
			ID: t.ID, SearchHash: t.SearchHash, OriginCity: t.OriginCity,
			Destination: t.Destination, HotelName: t.HotelName, HotelStars: t.HotelStars,
			Nights: t.Nights, Price: t.Price, Provider: t.Provider,
			TourURL: t.TourURL, AffiliateURL: t.AffiliateURL, UpdatedAt: t.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tours": tours})
}
type createSubscriptionRequest struct {
	OriginCity       string `json:"origin_city"`
	Destination      string `json:"destination"`
	DateFrom         string `json:"date_from"`
	DateTo           string `json:"date_to"`
	Nights           int    `json:"nights"`
	MaxPrice         int    `json:"max_price"`
	Messenger        string `json:"messenger"`
	UserIdentifier   string `json:"user_identifier"`
}

const (
	MessengerTelegram = "telegram"
	MessengerMax      = "max"
)

func (a *API) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.OriginCity == "" || req.Destination == "" {
		writeError(w, "Город вылета и направление обязательны", http.StatusBadRequest)
		return
	}
	if len(req.OriginCity) > 100 || len(req.Destination) > 100 {
		writeError(w, "Слишком длинные значения", http.StatusBadRequest)
		return
	}
	if req.Nights < 1 || req.Nights > 30 {
		req.Nights = 7
	}
	if req.MaxPrice < 0 {
		req.MaxPrice = 0
	}

	clientIP := sec.ExtractIP(r)
	count, _ := a.storage.CountRegistrationsTodayByIP(r.Context(), clientIP)
	if count >= a.cfg.MaxRegistrationsPerDayIP {
		writeError(w, "Превышен лимит (3 подписки в сутки с одного IP)", http.StatusTooManyRequests)
		return
	}
	subCount, _ := a.storage.CountActionsTodayByIP(r.Context(), clientIP, "subscribe")
	if subCount >= a.cfg.MaxSubscriptionsPerDayIP {
		writeError(w, "Превышен лимит подписок (10 в сутки с одного IP)", http.StatusTooManyRequests)
		return
	}

	if req.Messenger == "" {
		writeError(w, "Выберите мессенджер", http.StatusBadRequest)
		return
	}
	if req.Messenger != MessengerTelegram && req.Messenger != MessengerMax {
		writeError(w, "Неподдерживаемый мессенджер. Выберите Telegram или MAX", http.StatusBadRequest)
		return
	}
	if req.UserIdentifier == "" {
		writeError(w, "Укажите username или ID", http.StatusBadRequest)
		return
	}

	var userID int64
	if uid, ok := r.Context().Value(sec.UserIDKey).(int64); ok && uid > 0 {
		userID = uid
	} else if req.Messenger == MessengerTelegram {
		// Try to find user by identifier (chat_id or username)
		userID, _ = a.findUserByMessengerID(r.Context(), req.Messenger, req.UserIdentifier)
	}

	sub := &domain.Subscription{
		UserID:         userID,
		OriginCity:     req.OriginCity,
		Destination:    req.Destination,
		NightsMin:      req.Nights,
		NightsMax:      req.Nights,
		MaxPrice:       req.MaxPrice,
		Messenger:      req.Messenger,
		UserIdentifier: req.UserIdentifier,
		IsActive:       true,
	}
	if req.DateFrom != "" {
		if date, err := time.Parse("2006-01-02", req.DateFrom); err == nil {
			sub.DateFrom = date
		}
	}
	if req.DateTo != "" {
		if date, err := time.Parse("2006-01-02", req.DateTo); err == nil {
			sub.DateTo = date
		}
	}

	if err := a.storage.CreateSubscription(r.Context(), sub); err != nil {
		writeError(w, "Ошибка при создании подписки", http.StatusInternalServerError)
		return
	}
	_ = a.storage.RecordRateLimit(r.Context(), clientIP, "subscribe")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Подписка создана", "id": sub.ID})
}

func (a *API) handleMySubscriptions(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(sec.UserIDKey).(int64)
	subs, err := a.storage.GetUserActiveSubscriptions(r.Context(), userID)
	if err != nil {
		writeError(w, "Ошибка загрузки", http.StatusInternalServerError)
		return
	}
	type subResp struct {
		ID          int64     `json:"id"`
		OriginCity  string    `json:"origin_city"`
		Destination string    `json:"destination"`
		NightsMin   int       `json:"nights_min"`
		NightsMax   int       `json:"nights_max"`
		MaxPrice    int       `json:"max_price"`
		CreatedAt   time.Time `json:"created_at"`
	}
	resp := make([]subResp, 0, len(subs))
	for _, s := range subs {
		resp = append(resp, subResp{
			ID: s.ID, OriginCity: s.OriginCity, Destination: s.Destination,
			NightsMin: s.NightsMin, NightsMax: s.NightsMax, MaxPrice: s.MaxPrice, CreatedAt: s.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"subscriptions": resp})
}

func (a *API) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(sec.UserIDKey).(int64)
	subs, _ := a.storage.GetUserActiveSubscriptions(r.Context(), userID)
	subID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	for _, s := range subs {
		if s.ID == subID {
			_ = a.storage.DeactivateSubscription(r.Context(), subID)
			writeError(w, "Подписка удалена", http.StatusOK)
			return
		}
	}
	writeError(w, "Подписка не найдена", http.StatusNotFound)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(sec.UserIDKey).(int64)
	email, _ := r.Context().Value(sec.EmailKey).(string)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"user": map[string]interface{}{"id": userID, "email": email}})
}

func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (a *API) findUserByMessengerID(ctx context.Context, messenger, identifier string) (int64, error) {
	user, err := a.storage.FindUserByMessengerID(ctx, messenger, identifier)
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}