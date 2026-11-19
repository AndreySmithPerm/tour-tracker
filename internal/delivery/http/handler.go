package delivery

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/travel-to-tour/tour-tracker/internal/domain"
	sec "github.com/travel-to-tour/tour-tracker/internal/middleware"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// HTTP handler for web forms
type HTTP struct {
	storage *storage.Storage
	logger  *log.Logger
	tmpl    *template.Template
	cfg     sec.Config
}

// NewHTTP creates a new HTTP handler
func NewHTTP(s *storage.Storage, cfg sec.Config) *HTTP {
	return &HTTP{
		storage: s,
		logger:  log.Default(),
		tmpl:    nil,
		cfg:     cfg,
	}
}

// Router returns the configured chi router
func (h *HTTP) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Apply client request tracking middleware to ALL routes
	r.Use(sec.ClientRequestMiddleware(h.storage, h.cfg))

	// Public routes
	r.Get("/", h.handleIndex)
	r.Get("/health", h.handleHealth)

	// Protected routes (require Yandex Captcha + rate limits)
	r.Post("/subscribe", h.handleSubscribe)
	r.Get("/subscriptions", h.handleSubscriptions)

	return r
}

// --- Health check (no limit) ---

func (h *HTTP) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Index page with Yandex Captcha ---

func (h *HTTP) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tour Tracker — Отслеживание цен на туры</title>
    <script src="https://yandex.ru/recaptcha/config.js" async></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f7fa; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        h1 { text-align: center; margin-bottom: 10px; font-size: 28px; }
        .subtitle { text-align: center; color: #666; margin-bottom: 30px; }
        .card { background: white; border-radius: 12px; padding: 30px; box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 6px; font-weight: 600; font-size: 14px; }
        input, select { width: 100%; padding: 12px 16px; border: 2px solid #e1e5eb; border-radius: 8px; font-size: 16px; transition: border-color 0.2s; }
        input:focus, select:focus { outline: none; border-color: #4a90d9; }
        .row { display: flex; gap: 15px; }
        .row .form-group { flex: 1; }
        button { width: 100%; padding: 14px; background: #4a90d9; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
        button:hover { background: #357abd; }
        .info { background: #e8f4fd; border-radius: 8px; padding: 15px; margin-top: 20px; font-size: 14px; color: #2c5282; }
        .telegram-btn { display: block; text-align: center; margin-top: 12px; padding: 12px; background: #0088cc; color: white; text-decoration: none; border-radius: 8px; font-weight: 600; }
        .error { color: #e53e3e; margin-top: 10px; font-size: 14px; }
        .success { color: #38a169; margin-top: 10px; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✈️ Tour Tracker</h1>
        <p class="subtitle">Отслеживайте цены на туры и ловите лучшие предложения</p>
        
        <div class="card">
            <form id="subscribeForm" method="POST" action="/subscribe">
                <div class="form-group">
                    <label>Город вылета</label>
                    <input type="text" name="origin_city" placeholder="Москва" required maxlength="100">
                </div>
                
                <div class="form-group">
                    <label>Направление</label>
                    <input type="text" name="destination" placeholder="Анталья, Турция" required maxlength="100">
                </div>
                
                <div class="row">
                    <div class="form-group">
                        <label>Дата вылета</label>
                        <input type="date" name="date_from">
                    </div>
                    <div class="form-group">
                        <label>Кол-во ночей</label>
                        <input type="number" name="nights" value="7" min="1" max="30">
                    </div>
                </div>
                
                <div class="form-group">
                    <label>Макс. бюджет на человека (₽)</label>
                    <input type="number" name="max_price" placeholder="Не ограничено">
                </div>

                <!-- Yandex Captcha widget -->
                <div class="form-group">
                    <div class="yandex-recaptcha"
                         data-yandex-recaptcha="1"
                         data-widget="subscribe"
                         data-sitekey="{{.YandexCaptchaSiteKey}}">
                    </div>
                </div>
                
                <button type="submit">Добавить подписку</button>
                <div id="message"></div>
            </form>
            
            <div class="info">
                💡 Подпишитесь на наш Telegram-бот для мгновенных уведомлений: 
                <a href="{{.BotURL}}" class="telegram-btn" target="_blank">{{.BotHandle}}</a>
            </div>
        </div>
    </div>
</body>
</html>`

	t := template.Must(template.New("index").Parse(tmpl))
	t.Execute(w, map[string]string{
		"YandexCaptchaSiteKey": "", // Set from env
		"BotURL":               "", // Set from env
		"BotHandle":            "", // Set from env
	})
}

// --- Handle subscription creation with Captcha + validation ---

func (h *HTTP) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce subscription rate limit
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.createSubscription(w, r)
	})
	limited := sec.SubscriptionLimitMiddleware(h.storage, h.cfg)(next)
	limited.ServeHTTP(w, r)
}

func (h *HTTP) createSubscription(w http.ResponseWriter, r *http.Request) {
	// Enforce max body size (64KB)
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	// Parse form
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originCity := strings.TrimSpace(r.FormValue("origin_city"))
	destination := strings.TrimSpace(r.FormValue("destination"))
	dateFromStr := strings.TrimSpace(r.FormValue("date_from"))
	nightsStr := strings.TrimSpace(r.FormValue("nights"))
	maxPriceStr := strings.TrimSpace(r.FormValue("max_price"))

	// --- Validation ---
	if originCity == "" || destination == "" {
		http.Error(w, "Город вылета и направление обязательны", http.StatusBadRequest)
		return
	}
	if len(originCity) > 100 || len(destination) > 100 {
		http.Error(w, "Слишком длинные значения", http.StatusBadRequest)
		return
	}

	nights := 7
	if nightsStr != "" {
		n, err := strconv.Atoi(nightsStr)
		if err != nil || n < 1 || n > 30 {
			http.Error(w, "Неверное количество ночей (1-30)", http.StatusBadRequest)
			return
		}
		nights = n
	}

	maxPrice := 0
	if maxPriceStr != "" {
		p, err := strconv.Atoi(maxPriceStr)
		if err != nil || p < 0 {
			http.Error(w, "Неверный бюджет", http.StatusBadRequest)
			return
		}
		maxPrice = p
	}

	var dateFrom time.Time
	if dateFromStr != "" {
		dateFrom, err = time.Parse("2006-01-02", dateFromStr)
		if err != nil {
			http.Error(w, "Неверный формат даты (ГГГГ-ММ-ДД)", http.StatusBadRequest)
			return
		}
	}

	// --- Yandex Captcha verification ---
	captchaResponse := r.FormValue("yandex_captcha_response")
	if captchaResponse == "" {
		http.Error(w, "Подтвердите, что вы не робот", http.StatusBadRequest)
		return
	}

	// TODO: Verify captcha token with Yandex API
	// POST https://connect.yandex.ru/mobile/v1/token/verify
	// Body: {token: captchaResponse, secret: YANDEX_CAPTCHA_SECRET}
	// If verification fails, return 400

	// --- Extract client IP ---
	clientIP := sec.ExtractIP(r)

	// --- Check registration limit ---
	count, err := h.storage.CountRegistrationsTodayByIP(r.Context(), clientIP)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if count >= h.cfg.MaxRegistrationsPerDayIP {
		http.Error(w, "Превышен лимит регистраций (3 в сутки с одного IP)", http.StatusTooManyRequests)
		return
	}

	// --- Create subscription ---
	sub := &domain.Subscription{
		OriginCity:  originCity,
		Destination: destination,
		NightsMin:   nights,
		NightsMax:   nights,
		MaxPrice:    maxPrice,
		IsActive:    true,
	}

	if !dateFrom.IsZero() {
		sub.DateFrom = dateFrom
	}

	err = h.storage.CreateSubscription(r.Context(), sub)
	if err != nil {
		h.logger.Printf("CreateSubscription error: %v", err)
		http.Error(w, "Ошибка при создании подписки", http.StatusInternalServerError)
		return
	}

	// Record IP registration
	_ = h.storage.RecordIPRegistration(r.Context(), clientIP, nil)

	// Record rate limit action
	_ = h.storage.RecordRateLimit(r.Context(), clientIP, "subscribe")

	w.Header().Set("Location", "/?success=1")
	w.WriteHeader(http.StatusFound)
}

func (h *HTTP) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Мои подписки — Tour Tracker</title>
    <style>
        body { font-family: system-ui; max-width: 600px; margin: 40px auto; padding: 0 20px; }
        h1 { margin-bottom: 20px; }
        .empty { color: #666; padding: 40px; text-align: center; }
        a { color: #4a90d9; }
    </style>
</head>
<body>
    <h1>📋 Мои подписки</h1>
    <div class="empty">
        <p>Для управления подписками используйте <a href="{{.BotURL}}">{{.BotHandle}}</a></p>
        <p><a href="/">← На главную</a></p>
    </div>
</body>
</html>`

	t := template.Must(template.New("subs").Parse(tmpl))
	t.Execute(w, map[string]string{
		"BotURL":    "", // Set from env
		"BotHandle": "", // Set from env
	})
}
