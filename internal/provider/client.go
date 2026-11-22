package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/travel-to-tour/tour-tracker/internal/domain"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// TourProvider defines the interface for fetching tour offers
type TourProvider interface {
	Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error)
	Name() string
}

// StubProvider is a placeholder implementation for development
type StubProvider struct {
	name string
}

func NewStubProvider(name string) *StubProvider {
	return &StubProvider{name: name}
}

func (p *StubProvider) Name() string {
	return p.name
}

func (p *StubProvider) Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error) {
	offers := []*domain.TourOffer{
		{
			SearchHash:  storage.GenerateSearchHash(key.OriginCity, key.Destination, key.Country, key.MonthBucket),
			OriginCity:  key.OriginCity,
			Destination: key.Destination,
			HotelName:   "Sample Hotel " + key.Destination,
			HotelStars:  5,
			Nights:      7,
			Price:       85000,
			Provider:    p.name,
			TourURL:     fmt.Sprintf("https://example.com/tour/%s/%s", key.Destination, key.MonthBucket),
		},
		{
			SearchHash:  storage.GenerateSearchHash(key.OriginCity, key.Destination, key.Country, key.MonthBucket),
			OriginCity:  key.OriginCity,
			Destination: key.Destination,
			HotelName:   "Budget Hotel " + key.Destination,
			HotelStars:  3,
			Nights:      7,
			Price:       45000,
			Provider:    p.name,
			TourURL:     fmt.Sprintf("https://example.com/tour/%s/%s", key.Destination, key.MonthBucket),
		},
	}
	return offers, nil
}

// TravelataProvider integrates with Travelata API via Travelpayouts
type TravelataProvider struct {
	name       string
	apiKey     string
	marker     string
	httpClient *http.Client
}

func NewTravelataProvider(apiKey, marker string) *TravelataProvider {
	return &TravelataProvider{
		name:       "travelata",
		apiKey:     apiKey,
		marker:     marker,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *TravelataProvider) Name() string {
	return "travelata"
}

func (p *TravelataProvider) Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error) {
	// Travelpayouts API v1 search - GET with query params
	// https://www.travelpayouts.ru/developers/search-tickets
	baseURL := "https://api.travelpayouts.com/api/v1/search"

	var year, month int
	_, err := fmt.Sscanf(key.MonthBucket, "%d-%d", &year, &month)
	if err != nil || year == 0 || month == 0 {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}
	searchDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	returnDate := searchDate.AddDate(0, 0, 7)

	// Build query parameters
	params := url.Values{}
	params.Add("token", p.apiKey)
	params.Add("origin", key.OriginCity)
	params.Add("destination", key.Destination)
	params.Add("depart_date", searchDate.Format("2006-01-02"))
	params.Add("return_date", returnDate.Format("2006-01-02"))
	params.Add("currency", "rub")
	params.Add("trip_class", "0")
	params.Add("show_to_affiliates", "true")
	params.Add("adults", "1")

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp []struct {
		DepartDate   string  `json:"depart_date"`
		ReturnDate   string  `json:"return_date"`
		Nights       int     `json:"nights"`
		Price        int     `json:"price"`
		Origin       string  `json:"origin"`
		Destination  string  `json:"destination"`
		HotelName    string  `json:"hotel_name"`
		HotelStars   int     `json:"hotel_stars"`
		Transfers    struct {
			Take   string `json:"take"`
			Return string `json:"return"`
		} `json:"transfers"`
		Link string `json:"link"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var offers []*domain.TourOffer
	for _, round := range apiResp {
		if round.Price <= 0 {
			continue
		}
		tourURL := round.Link
		if tourURL == "" {
			tourURL = fmt.Sprintf("https://www.travelata.ru/hotels/%s/%s?marker=%s",
				key.Destination, key.MonthBucket, p.marker)
		}
		offers = append(offers, &domain.TourOffer{
			SearchHash:  storage.GenerateSearchHash(key.OriginCity, key.Destination, key.Country, key.MonthBucket),
			OriginCity:  round.Origin,
			Destination: round.Destination,
			HotelName:   round.HotelName,
			HotelStars:  round.HotelStars,
			Nights:      round.Nights,
			Price:       round.Price,
			Provider:    p.name,
			TourURL:     tourURL,
		})
	}

	return offers, nil
}

// LevelProvider would integrate with Level.travel API
type LevelProvider struct {
	apiKey     string
	marker     string
	httpClient *http.Client
}

func NewLevelProvider(apiKey, marker string) *LevelProvider {
	return &LevelProvider{
		apiKey: apiKey, marker: marker,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *LevelProvider) Name() string { return "level" }

func (p *LevelProvider) Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error) {
	return nil, fmt.Errorf("not implemented: Level.travel API integration pending provider access")
}
