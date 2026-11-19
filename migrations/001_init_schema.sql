-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    tg_chat_id BIGINT NOT NULL,
    tg_username VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    origin_city VARCHAR(100) NOT NULL,
    destination VARCHAR(100) NOT NULL,
    country VARCHAR(100),
    date_from DATE,
    date_to DATE,
    nights_min INT NOT NULL DEFAULT 7,
    nights_max INT NOT NULL DEFAULT 14,
    max_price INT,
    min_price_drop NUMERIC(5,4) NOT NULL DEFAULT 0.10,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Price snapshots table (last known min price per search key)
CREATE TABLE IF NOT EXISTS price_snapshots (
    search_hash VARCHAR(255) PRIMARY KEY,
    min_price INT NOT NULL,
    tour_url TEXT,
    affiliate_url TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Tour offers table (full history)
CREATE TABLE IF NOT EXISTS tour_offers (
    id BIGSERIAL PRIMARY KEY,
    search_hash VARCHAR(255) NOT NULL,
    origin_city VARCHAR(100) NOT NULL,
    destination VARCHAR(100) NOT NULL,
    hotel_name VARCHAR(255),
    hotel_stars INT,
    nights INT,
    price INT NOT NULL,
    provider VARCHAR(50) NOT NULL,
    tour_url TEXT NOT NULL,
    affiliate_url TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Sent notifications (deduplication)
CREATE TABLE IF NOT EXISTS sent_notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tour_hash VARCHAR(255) NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- IP registration tracking (anti-spam)
CREATE TABLE IF NOT EXISTS ip_registrations (
    id BIGSERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL,  -- IPv4 or IPv6
    user_id BIGINT REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ip_registrations_ip ON ip_registrations(ip_address, created_at);

-- Rate limiting for subscription creation (per IP)
CREATE TABLE IF NOT EXISTS ip_rate_limits (
    id BIGSERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL,
    action VARCHAR(50) NOT NULL,  -- 'subscribe', 'link_generate'
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ip_rate_limits_ip_action ON ip_rate_limits(ip_address, action, created_at);

-- Client requests tracking (daily limit per IP)
CREATE TABLE IF NOT EXISTS client_requests (
    id BIGSERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL,
    path VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    user_id BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_client_requests_ip ON client_requests(ip_address, created_at);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_subscriptions_active ON subscriptions(is_active, origin_city, destination, date_from);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_tour_offers_search ON tour_offers(search_hash);
CREATE INDEX IF NOT EXISTS idx_sent_notifications_user ON sent_notifications(user_id, tour_hash);
