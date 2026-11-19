package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/travel-to-tour/tour-tracker/internal/domain"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// TourProvider defines the interface for fetching tour offers
type TourProvider interface {
	// Search finds tour offers for a given search key
	Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error)
	// Name returns the provider name (e.g. "travelata", "level")
	Name() string
}

// SearchParams holds parameters for a tour search
type SearchParams struct {
	OriginCity  string
	Destination string
	Country     string
	MonthBucket string // e.g. "2026-07"
	NightsMin   int
	NightsMax   int
}

// StubProvider is a placeholder implementation for development
// Replace with actual Travelata/Level.travel API integration
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
	// TODO: Replace with actual API call to Travelata or Level.travel
	// This is a stub that returns sample data for development

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

// TravelataProvider would integrate with Travelata API
type TravelataProvider struct {
	apiKey    string
	marker    string
	httpClient *http.Client
}

func NewTravelataProvider(apiKey, marker string) *TravelataProvider {
	return &TravelataProvider{
		apiKey: apiKey,
		marker: marker,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *TravelataProvider) Name() string {
	return "travelata"
}

func (p *TravelataProvider) Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error) {
	// TODO: Implement actual Travelata API integration
	// Reference: https://www.travelpayouts.ru/developers/packaged-tours-methods
	return nil, fmt.Errorf("not implemented: Travelata API integration pending provider access")
}

// LevelProvider would integrate with Level.travel API
type LevelProvider struct {
	apiKey    string
	marker    string
	httpClient *http.Client
}

func NewLevelProvider(apiKey, marker string) *LevelProvider {
	return &LevelProvider{
		apiKey: apiKey,
		marker: marker,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *LevelProvider) Name() string {
	return "level"
}

func (p *LevelProvider) Search(ctx context.Context, key domain.SearchKey) ([]*domain.TourOffer, error) {
	// TODO: Implement actual Level.travel API integration
	return nil, fmt.Errorf("not implemented: Level.travel API integration pending provider access")
}
