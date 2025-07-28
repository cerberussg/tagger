// pkg/enricher/musicbrainz/musicbrainz.go

package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cerberussg/tagger/pkg/enricher"
)

const (
	baseURL     = "https://musicbrainz.org/ws/2"
	userAgent   = "tagger/0.1.0 (https://github.com/cerberussg/tagger)"
	rateLimit   = time.Second // 1 request per second
)

// MusicBrainzProvider implements the MetadataProvider interface for MusicBrainz
type MusicBrainzProvider struct {
	client      *http.Client
	userAgent   string
	lastRequest time.Time
}

// NewMusicBrainzProvider creates a new MusicBrainz metadata provider
func NewMusicBrainzProvider() *MusicBrainzProvider {
	return &MusicBrainzProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}
}

// Name returns the provider's display name
func (m *MusicBrainzProvider) Name() string {
	return "MusicBrainz"
}

// Lookup searches for track metadata by artist and title
func (m *MusicBrainzProvider) Lookup(ctx context.Context, artist, title string) (*enricher.TrackMetadata, error) {
	req := &enricher.SearchRequest{
		Artist:                artist,
		Title:                 title,
		PreferOriginalRelease: true,
		MaxResults:           5,
	}
	return m.LookupWithHints(ctx, req)
}

// LookupWithHints performs advanced search with additional parameters
func (m *MusicBrainzProvider) LookupWithHints(ctx context.Context, req *enricher.SearchRequest) (*enricher.TrackMetadata, error) {
	// Rate limiting - ensure we don't exceed 1 req/sec
	if err := m.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	// Search for recordings first
	recordings, err := m.searchRecordings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz recording search failed: %w", err)
	}

	if len(recordings) == 0 {
		return nil, enricher.ErrNotFound
	}

	// Get the best recording match
	bestRecording := m.findBestRecordingMatch(recordings, req.Artist, req.Title)
	if bestRecording == nil {
		return nil, enricher.ErrNotFound
	}

	// Get release information for the recording
	releases, err := m.getRecordingReleases(ctx, bestRecording.ID)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz release lookup failed: %w", err)
	}

	// Find the best release (prefer original releases)
	bestRelease := m.findBestRelease(releases, req.PreferOriginalRelease)
	if bestRelease == nil {
		return nil, enricher.ErrNotFound
	}

	// Convert to our standard format
	metadata := m.convertToTrackMetadata(bestRecording, bestRelease, req.Artist, req.Title)
	return metadata, nil
}

// SupportsGenre indicates if MusicBrainz has good coverage for a genre
func (m *MusicBrainzProvider) SupportsGenre(genre string) bool {
	// MusicBrainz has good coverage for most genres, especially established ones
	genre = strings.ToLower(genre)
	switch genre {
	case "dnb", "drum and bass", "jungle":
		return true
	case "electronic", "house", "techno", "trance":
		return true
	case "rock", "pop", "jazz", "classical":
		return true
	default:
		return true // Generally good coverage
	}
}

// RateLimit returns the provider's rate limiting info
func (m *MusicBrainzProvider) RateLimit() enricher.RateLimitInfo {
	return enricher.RateLimitInfo{
		RequestsPerSecond: 1.0,
		BurstAllowed:      1,
		RequiresUserAgent: true,
		RequiresAPIKey:    false,
	}
}

// Close cleans up any resources
func (m *MusicBrainzProvider) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}

// waitForRateLimit enforces the 1 req/sec rate limit
func (m *MusicBrainzProvider) waitForRateLimit(ctx context.Context) error {
	elapsed := time.Since(m.lastRequest)
	if elapsed < rateLimit {
		waitTime := rateLimit - elapsed
		
		select {
		case <-time.After(waitTime):
			// Wait completed successfully
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	m.lastRequest = time.Now()
	return nil
}

// searchRecordings searches MusicBrainz for recordings matching the criteria
func (m *MusicBrainzProvider) searchRecordings(ctx context.Context, req *enricher.SearchRequest) ([]Recording, error) {
	// Build search query
	query := fmt.Sprintf(`artist:"%s" AND recording:"%s"`, req.Artist, req.Title)
	
	// Add additional hints if available
	if req.Album != "" {
		query += fmt.Sprintf(` AND release:"%s"`, req.Album)
	}

	// Prepare URL
	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", strconv.Itoa(req.MaxResults))
	params.Set("fmt", "json")
	
	searchURL := fmt.Sprintf("%s/recording?%s", baseURL, params.Encode())

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("User-Agent", m.userAgent)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz API returned status %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var searchResult RecordingSearchResult
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return searchResult.Recordings, nil
}

// getRecordingReleases fetches release information for a recording
func (m *MusicBrainzProvider) getRecordingReleases(ctx context.Context, recordingID string) ([]Release, error) {
	if err := m.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	// Get releases for this recording
	params := url.Values{}
	params.Set("inc", "labels")
	params.Set("fmt", "json")
	
	lookupURL := fmt.Sprintf("%s/recording/%s?%s", baseURL, recordingID, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", lookupURL, nil)
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("User-Agent", m.userAgent)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var recording RecordingDetail
	if err := json.Unmarshal(body, &recording); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return recording.Releases, nil
}

// findBestRecordingMatch finds the recording that best matches the search criteria
func (m *MusicBrainzProvider) findBestRecordingMatch(recordings []Recording, targetArtist, targetTitle string) *Recording {
	if len(recordings) == 0 {
		return nil
	}

	// Simple scoring: prefer exact matches, then by score
	bestScore := 0
	var bestRecording *Recording

	for i, recording := range recordings {
		score := recording.Score
		
		// Bonus for exact title match
		if strings.EqualFold(recording.Title, targetTitle) {
			score += 10
		}
		
		// Bonus for exact artist match
		for _, credit := range recording.ArtistCredit {
			if strings.EqualFold(credit.Artist.Name, targetArtist) {
				score += 10
				break
			}
		}

		if score > bestScore {
			bestScore = score
			bestRecording = &recordings[i]
		}
	}

	return bestRecording
}

// findBestRelease finds the best release from a list, preferring original releases
func (m *MusicBrainzProvider) findBestRelease(releases []Release, preferOriginal bool) *Release {
	if len(releases) == 0 {
		return nil
	}

	if !preferOriginal {
		return &releases[0] // Just return the first one
	}

	// Prefer releases with earlier dates (likely originals)
	var bestRelease *Release
	var earliestDate string

	for i, release := range releases {
		if release.Date != "" {
			if earliestDate == "" || release.Date < earliestDate {
				earliestDate = release.Date
				bestRelease = &releases[i]
			}
		} else if bestRelease == nil {
			bestRelease = &releases[i] // Fallback to first release with no date
		}
	}

	if bestRelease == nil && len(releases) > 0 {
		bestRelease = &releases[0] // Ultimate fallback
	}

	return bestRelease
}

// convertToTrackMetadata converts MusicBrainz data to our standard format
func (m *MusicBrainzProvider) convertToTrackMetadata(recording *Recording, release *Release, originalArtist, originalTitle string) *enricher.TrackMetadata {
	metadata := &enricher.TrackMetadata{
		Artist:       originalArtist, // Use the original parsed artist
		Title:        originalTitle,  // Use the original parsed title
		Album:        release.Title,
		ReleaseDate:  release.Date,
		ProviderID:   recording.ID,
		ProviderName: "MusicBrainz",
		Extra:        make(map[string]interface{}),
	}

	// Extract year from date
	if release.Date != "" && len(release.Date) >= 4 {
		if year, err := strconv.Atoi(release.Date[:4]); err == nil {
			metadata.Year = year
		}
	}

	// Extract label information
	if len(release.LabelInfo) > 0 {
		metadata.Label = release.LabelInfo[0].Label.Name
		if release.LabelInfo[0].CatalogNumber != "" {
			metadata.CatalogNumber = release.LabelInfo[0].CatalogNumber
		}
	}

	// Calculate confidence based on match quality and completeness
	exactArtistMatch := false
	exactTitleMatch := strings.EqualFold(recording.Title, originalTitle)
	
	for _, credit := range recording.ArtistCredit {
		if strings.EqualFold(credit.Artist.Name, originalArtist) {
			exactArtistMatch = true
			break
		}
	}

	metadata.Confidence = enricher.CalculateConfidence(metadata, exactArtistMatch && exactTitleMatch)

	// Store additional MusicBrainz-specific data
	metadata.Extra["musicbrainz_recording_id"] = recording.ID
	metadata.Extra["musicbrainz_release_id"] = release.ID
	metadata.Extra["musicbrainz_score"] = recording.Score

	return metadata
}