// pkg/enricher/enricher.go - Core interfaces and orchestrator

package enricher

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrNotFound    = errors.New("no metadata found")
	ErrRateLimit   = errors.New("rate limit exceeded")
	ErrAPIError    = errors.New("API error")
	ErrNoProvider  = errors.New("no providers available")
)

// MetadataProvider is the core interface that all API adapters implement
type MetadataProvider interface {
	// Name returns the provider's display name (e.g., "MusicBrainz", "Discogs")
	Name() string
	
	// Lookup searches for track metadata by artist and title
	Lookup(ctx context.Context, artist, title string) (*TrackMetadata, error)
	
	// LookupWithHints allows passing additional search hints
	LookupWithHints(ctx context.Context, req *SearchRequest) (*TrackMetadata, error)
	
	// SupportsGenre indicates if this provider has good coverage for a genre
	SupportsGenre(genre string) bool
	
	// RateLimit returns the provider's rate limiting info
	RateLimit() RateLimitInfo
	
	// Close cleans up any resources (connections, caches, etc.)
	Close() error
}

// TrackMetadata represents the enriched metadata from any provider
type TrackMetadata struct {
	Artist        string            `json:"artist"`
	Title         string            `json:"title"`
	Album         string            `json:"album,omitempty"`
	Label         string            `json:"label,omitempty"`
	ReleaseDate   string            `json:"release_date,omitempty"`
	Genre         string            `json:"genre,omitempty"`
	CatalogNumber string            `json:"catalog_number,omitempty"`
	Year          int               `json:"year,omitempty"`
	
	// Provider-specific data
	ProviderID    string            `json:"provider_id"`    // e.g., MusicBrainz MBID
	ProviderName  string            `json:"provider_name"`  // e.g., "MusicBrainz"
	Confidence    float64           `json:"confidence"`     // 0.0 - 1.0
	
	// Extensible fields for provider-specific data
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// SearchRequest contains all possible search parameters
type SearchRequest struct {
	Artist      string
	Title       string
	Album       string
	Genre       string
	Year        string
	Duration    time.Duration
	
	// Search preferences
	PreferOriginalRelease bool
	MaxResults           int
}

// RateLimitInfo describes the provider's rate limiting
type RateLimitInfo struct {
	RequestsPerSecond float64
	BurstAllowed      int
	RequiresUserAgent bool
	RequiresAPIKey    bool
}

// ProviderStrategy defines how to use multiple providers
type ProviderStrategy string

const (
	StrategyFirst     ProviderStrategy = "first"      // Use first successful result
	StrategyBest      ProviderStrategy = "best"       // Compare confidence scores
	StrategyFallback  ProviderStrategy = "fallback"   // Try in priority order
)

// EnricherConfig holds configuration for the enricher
type EnricherConfig struct {
	// Provider selection strategy
	Strategy ProviderStrategy `yaml:"strategy"`
	
	// Quality thresholds
	MinConfidence     float64       `yaml:"min_confidence"`
	RequireLabel      bool          `yaml:"require_label"`
	
	// Timeouts
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	
	// For future use
	CacheEnabled      bool          `yaml:"cache_enabled"`
	CacheTTL          time.Duration `yaml:"cache_ttl"`
}

// Enricher orchestrates multiple metadata providers
type Enricher struct {
	providers []MetadataProvider
	config    *EnricherConfig
}

// NewEnricher creates an enricher with the specified providers
func NewEnricher(providers []MetadataProvider, config *EnricherConfig) *Enricher {
	if config == nil {
		config = &EnricherConfig{
			Strategy:       StrategyFirst,
			MinConfidence:  0.7,
			RequireLabel:   false,
			RequestTimeout: 30 * time.Second,
			CacheEnabled:   true,
			CacheTTL:       24 * time.Hour,
		}
	}
	
	return &Enricher{
		providers: providers,
		config:    config,
	}
}

// Lookup finds metadata using the configured strategy
func (e *Enricher) Lookup(ctx context.Context, artist, title string) (*TrackMetadata, error) {
	if len(e.providers) == 0 {
		return nil, ErrNoProvider
	}
	
	req := &SearchRequest{
		Artist:                artist,
		Title:                 title,
		PreferOriginalRelease: true,
		MaxResults:           5,
	}
	
	return e.LookupWithRequest(ctx, req)
}

// LookupWithRequest performs lookup with full search parameters
func (e *Enricher) LookupWithRequest(ctx context.Context, req *SearchRequest) (*TrackMetadata, error) {
	// Apply request timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()
	
	switch e.config.Strategy {
	case StrategyFirst:
		return e.lookupFirst(ctx, req)
	case StrategyBest:
		return e.lookupBest(ctx, req)
	case StrategyFallback:
		return e.lookupFallback(ctx, req)
	default:
		return e.lookupFirst(ctx, req)
	}
}

// lookupFirst tries providers in order, returns first successful result
func (e *Enricher) lookupFirst(ctx context.Context, req *SearchRequest) (*TrackMetadata, error) {
	var lastErr error
	
	for _, provider := range e.providers {
		result, err := provider.LookupWithHints(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}
		
		if result != nil && result.Confidence >= e.config.MinConfidence {
			if !e.config.RequireLabel || result.Label != "" {
				return result, nil
			}
		}
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNotFound
}

// lookupBest tries all providers and returns the best result by confidence
func (e *Enricher) lookupBest(ctx context.Context, req *SearchRequest) (*TrackMetadata, error) {
	var bestResult *TrackMetadata
	var lastErr error
	
	for _, provider := range e.providers {
		result, err := provider.LookupWithHints(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}
		
		if result != nil && result.Confidence >= e.config.MinConfidence {
			if !e.config.RequireLabel || result.Label != "" {
				if bestResult == nil || result.Confidence > bestResult.Confidence {
					bestResult = result
				}
			}
		}
	}
	
	if bestResult != nil {
		return bestResult, nil
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNotFound
}

// lookupFallback tries providers in order with more aggressive fallback
func (e *Enricher) lookupFallback(ctx context.Context, req *SearchRequest) (*TrackMetadata, error) {
	// First pass: try with all hints
	result, err := e.lookupFirst(ctx, req)
	if err == nil && result != nil {
		return result, nil
	}
	
	// Second pass: try with simplified search (artist + title only)
	simplifiedReq := &SearchRequest{
		Artist:                req.Artist,
		Title:                 req.Title,
		PreferOriginalRelease: req.PreferOriginalRelease,
		MaxResults:           req.MaxResults,
	}
	
	return e.lookupFirst(ctx, simplifiedReq)
}

// AddProvider adds a new provider to the enricher
func (e *Enricher) AddProvider(provider MetadataProvider) {
	e.providers = append(e.providers, provider)
}

// GetProviders returns all registered providers
func (e *Enricher) GetProviders() []MetadataProvider {
	return e.providers
}

// Close closes all providers
func (e *Enricher) Close() error {
	var lastErr error
	for _, provider := range e.providers {
		if err := provider.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Helper function to calculate confidence score based on completeness
func CalculateConfidence(metadata *TrackMetadata, exactMatch bool) float64 {
	confidence := 0.0
	
	// Base score for finding anything
	confidence += 0.2
	
	// Exact vs fuzzy match bonus
	if exactMatch {
		confidence += 0.4
	} else {
		confidence += 0.2
	}
	
	// Completeness bonuses
	if metadata.Label != "" {
		confidence += 0.2
	}
	if metadata.ReleaseDate != "" || metadata.Year > 0 {
		confidence += 0.1
	}
	if metadata.CatalogNumber != "" {
		confidence += 0.1
	}
	
	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return confidence
}