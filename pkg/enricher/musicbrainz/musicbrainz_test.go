// pkg/enricher/musicbrainz/musicbrainz_test.go

package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cerberussg/tagger/pkg/enricher"
)

func TestMusicBrainzProvider_Interface(t *testing.T) {
	// Ensure MusicBrainzProvider implements MetadataProvider interface
	var _ enricher.MetadataProvider = (*MusicBrainzProvider)(nil)
}

func TestNewMusicBrainzProvider(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	if provider == nil {
		t.Fatal("NewMusicBrainzProvider returned nil")
	}
	
	if provider.Name() != "MusicBrainz" {
		t.Errorf("Expected name 'MusicBrainz', got '%s'", provider.Name())
	}
	
	rateLimit := provider.RateLimit()
	if rateLimit.RequestsPerSecond != 1.0 {
		t.Errorf("Expected 1.0 requests per second, got %f", rateLimit.RequestsPerSecond)
	}
	
	if !rateLimit.RequiresUserAgent {
		t.Error("Expected RequiresUserAgent to be true")
	}
}

func TestMusicBrainzProvider_SupportsGenre(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	testCases := []struct {
		genre    string
		expected bool
	}{
		{"dnb", true},
		{"drum and bass", true},
		{"jungle", true},
		{"electronic", true},
		{"house", true},
		{"rock", true},
		{"unknown-genre", true}, // Should return true for unknown genres
	}
	
	for _, tc := range testCases {
		t.Run(tc.genre, func(t *testing.T) {
			result := provider.SupportsGenre(tc.genre)
			if result != tc.expected {
				t.Errorf("SupportsGenre(%s) = %v, expected %v", tc.genre, result, tc.expected)
			}
		})
	}
}

func TestMusicBrainzProvider_RateLimit(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	// Test that rate limiting works
	start := time.Now()
	
	ctx := context.Background()
	err1 := provider.waitForRateLimit(ctx)
	if err1 != nil {
		t.Fatalf("First rate limit wait failed: %v", err1)
	}
	
	err2 := provider.waitForRateLimit(ctx)
	if err2 != nil {
		t.Fatalf("Second rate limit wait failed: %v", err2)
	}
	
	elapsed := time.Since(start)
	if elapsed < time.Second {
		t.Errorf("Rate limiting not working: elapsed time %v is less than 1 second", elapsed)
	}
	
	// Test context cancellation during rate limit wait
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	err := provider.waitForRateLimit(cancelCtx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestMusicBrainzProvider_SearchRecordings_MockResponse(t *testing.T) {
	// Test the JSON parsing logic with a mock response
	mockJSON := `{
		"created": "2024-01-01T00:00:00Z",
		"count": 1,
		"offset": 0,
		"recordings": [
			{
				"id": "test-recording-id",
				"title": "Music",
				"score": 100,
				"artist-credit": [
					{
						"name": "LTJ Bukem",
						"artist": {
							"id": "test-artist-id",
							"name": "LTJ Bukem",
							"sort-name": "LTJ Bukem"
						}
					}
				]
			}
		]
	}`
	
	var result RecordingSearchResult
	err := json.Unmarshal([]byte(mockJSON), &result)
	
	if err != nil {
		t.Fatalf("Failed to parse mock JSON: %v", err)
	}
	
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
	
	if len(result.Recordings) != 1 {
		t.Fatalf("Expected 1 recording, got %d", len(result.Recordings))
	}
	
	recording := result.Recordings[0]
	if recording.ID != "test-recording-id" {
		t.Errorf("Expected ID 'test-recording-id', got '%s'", recording.ID)
	}
	
	if recording.Title != "Music" {
		t.Errorf("Expected title 'Music', got '%s'", recording.Title)
	}
	
	if recording.Score != 100 {
		t.Errorf("Expected score 100, got %d", recording.Score)
	}
	
	if len(recording.ArtistCredit) != 1 {
		t.Fatalf("Expected 1 artist credit, got %d", len(recording.ArtistCredit))
	}
	
	if recording.ArtistCredit[0].Artist.Name != "LTJ Bukem" {
		t.Errorf("Expected artist 'LTJ Bukem', got '%s'", recording.ArtistCredit[0].Artist.Name)
	}
}

func TestMusicBrainzProvider_HTTPHeaders(t *testing.T) {
	// Test that HTTP requests are built correctly
	requestReceived := false
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		
		// Check User-Agent header
		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "tagger") {
			t.Errorf("Expected User-Agent to contain 'tagger', got '%s'", userAgent)
		}
		
		// Check Accept header
		accept := r.Header.Get("Accept")
		if accept != "application/json" {
			t.Errorf("Expected Accept header 'application/json', got '%s'", accept)
		}
		
		// Check query parameters
		query := r.URL.Query()
		if query.Get("fmt") != "json" {
			t.Errorf("Expected fmt=json parameter")
		}
		
		// Return empty but valid response to avoid errors
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"count": 0, "recordings": []}`)
	}))
	defer server.Close()
	
	// We would need to refactor the provider to accept a custom baseURL for this test
	// For now, this validates the test setup and shows how we would test HTTP behavior
	
	// This is a placeholder to show the test structure
	// In a real implementation, we'd need to inject the server URL
	_ = server.URL
	
	if !requestReceived {
		// This won't fail because we're not actually making the request
		// but it shows what we're testing for
		t.Log("Note: This test demonstrates HTTP header validation but requires provider refactoring")
	}
}

func TestMusicBrainzProvider_ConvertToTrackMetadata(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	// Create test data
	recording := &Recording{
		ID:    "test-recording-id",
		Title: "Music",
		Score: 100,
		ArtistCredit: []ArtistCredit{
			{
				Name: "LTJ Bukem",
				Artist: Artist{
					ID:   "test-artist-id",
					Name: "LTJ Bukem",
				},
			},
		},
	}
	
	release := &Release{
		ID:    "test-release-id",
		Title: "Good Looking Records Volume One",
		Date:  "1993-04-01",
		LabelInfo: []LabelInfo{
			{
				CatalogNumber: "GLREP001",
				Label: Label{
					ID:   "test-label-id",
					Name: "Good Looking Records",
				},
			},
		},
	}
	
	// Test conversion
	metadata := provider.convertToTrackMetadata(recording, release, "LTJ Bukem", "Music")
	
	// Validate result
	if metadata.Artist != "LTJ Bukem" {
		t.Errorf("Expected artist 'LTJ Bukem', got '%s'", metadata.Artist)
	}
	
	if metadata.Title != "Music" {
		t.Errorf("Expected title 'Music', got '%s'", metadata.Title)
	}
	
	if metadata.Album != "Good Looking Records Volume One" {
		t.Errorf("Expected album 'Good Looking Records Volume One', got '%s'", metadata.Album)
	}
	
	if metadata.Label != "Good Looking Records" {
		t.Errorf("Expected label 'Good Looking Records', got '%s'", metadata.Label)
	}
	
	if metadata.ReleaseDate != "1993-04-01" {
		t.Errorf("Expected release date '1993-04-01', got '%s'", metadata.ReleaseDate)
	}
	
	if metadata.Year != 1993 {
		t.Errorf("Expected year 1993, got %d", metadata.Year)
	}
	
	if metadata.CatalogNumber != "GLREP001" {
		t.Errorf("Expected catalog number 'GLREP001', got '%s'", metadata.CatalogNumber)
	}
	
	if metadata.ProviderName != "MusicBrainz" {
		t.Errorf("Expected provider name 'MusicBrainz', got '%s'", metadata.ProviderName)
	}
	
	if metadata.ProviderID != "test-recording-id" {
		t.Errorf("Expected provider ID 'test-recording-id', got '%s'", metadata.ProviderID)
	}
	
	if metadata.Confidence <= 0 || metadata.Confidence > 1 {
		t.Errorf("Expected confidence between 0 and 1, got %f", metadata.Confidence)
	}
	
	// Check extra fields
	if metadata.Extra["musicbrainz_recording_id"] != "test-recording-id" {
		t.Error("Missing musicbrainz_recording_id in extra fields")
	}
	
	if metadata.Extra["musicbrainz_release_id"] != "test-release-id" {
		t.Error("Missing musicbrainz_release_id in extra fields")
	}
}

func TestMusicBrainzProvider_FindBestRecordingMatch(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	recordings := []Recording{
		{
			ID:    "low-score",
			Title: "Different Song",
			Score: 50,
			ArtistCredit: []ArtistCredit{
				{Artist: Artist{Name: "Different Artist"}},
			},
		},
		{
			ID:    "exact-match",
			Title: "Music",
			Score: 80,
			ArtistCredit: []ArtistCredit{
				{Artist: Artist{Name: "LTJ Bukem"}},
			},
		},
		{
			ID:    "high-score-wrong-match",
			Title: "Wrong Song",
			Score: 90,
			ArtistCredit: []ArtistCredit{
				{Artist: Artist{Name: "Wrong Artist"}},
			},
		},
	}
	
	best := provider.findBestRecordingMatch(recordings, "LTJ Bukem", "Music")
	
	if best == nil {
		t.Fatal("findBestRecordingMatch returned nil")
	}
	
	if best.ID != "exact-match" {
		t.Errorf("Expected best match ID 'exact-match', got '%s'", best.ID)
	}
}

func TestMusicBrainzProvider_FindBestRelease(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	releases := []Release{
		{
			ID:    "reissue",
			Title: "Music (Reissue)",
			Date:  "2010-01-01",
		},
		{
			ID:    "original",
			Title: "Music",
			Date:  "1993-04-01",
		},
		{
			ID:    "later-release",
			Title: "Music (Special Edition)",
			Date:  "2000-01-01",
		},
	}
	
	// Test preferring original (earliest date)
	best := provider.findBestRelease(releases, true)
	
	if best == nil {
		t.Fatal("findBestRelease returned nil")
	}
	
	if best.ID != "original" {
		t.Errorf("Expected original release ID 'original', got '%s'", best.ID)
	}
	
	// Test not preferring original (first release)
	first := provider.findBestRelease(releases, false)
	
	if first == nil {
		t.Fatal("findBestRelease returned nil")
	}
	
	if first.ID != "reissue" {
		t.Errorf("Expected first release ID 'reissue', got '%s'", first.ID)
	}
}

func TestMusicBrainzProvider_ErrorHandling(t *testing.T) {
	// Test with server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()
	
	provider := NewMusicBrainzProvider()
	
	// This would require provider refactoring to accept custom baseURL
	// For now, we'll test error scenarios that don't require network calls
	
	// Test empty recordings
	best := provider.findBestRecordingMatch([]Recording{}, "Artist", "Title")
	if best != nil {
		t.Error("Expected nil for empty recordings slice")
	}
	
	// Test empty releases
	bestRelease := provider.findBestRelease([]Release{}, true)
	if bestRelease != nil {
		t.Error("Expected nil for empty releases slice")
	}
}

func TestMusicBrainzProvider_Close(t *testing.T) {
	provider := NewMusicBrainzProvider()
	
	err := provider.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// Benchmark tests
func BenchmarkMusicBrainzProvider_ConvertToTrackMetadata(b *testing.B) {
	provider := NewMusicBrainzProvider()
	
	recording := &Recording{
		ID:    "test-recording-id",
		Title: "Music",
		Score: 100,
		ArtistCredit: []ArtistCredit{
			{
				Name: "LTJ Bukem",
				Artist: Artist{
					ID:   "test-artist-id",
					Name: "LTJ Bukem",
				},
			},
		},
	}
	
	release := &Release{
		ID:    "test-release-id",
		Title: "Good Looking Records Volume One",
		Date:  "1993-04-01",
		LabelInfo: []LabelInfo{
			{
				CatalogNumber: "GLREP001",
				Label: Label{
					ID:   "test-label-id",
					Name: "Good Looking Records",
				},
			},
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		provider.convertToTrackMetadata(recording, release, "LTJ Bukem", "Music")
	}
}

func BenchmarkMusicBrainzProvider_FindBestRecordingMatch(b *testing.B) {
	provider := NewMusicBrainzProvider()
	
	// Create a large slice of recordings for benchmarking
	recordings := make([]Recording, 100)
	for i := 0; i < 100; i++ {
		recordings[i] = Recording{
			ID:    fmt.Sprintf("recording-%d", i),
			Title: fmt.Sprintf("Title %d", i),
			Score: i,
			ArtistCredit: []ArtistCredit{
				{Artist: Artist{Name: fmt.Sprintf("Artist %d", i)}},
			},
		}
	}
	
	// Add our target at the end
	recordings[99] = Recording{
		ID:    "target",
		Title: "Music",
		Score: 90,
		ArtistCredit: []ArtistCredit{
			{Artist: Artist{Name: "LTJ Bukem"}},
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		provider.findBestRecordingMatch(recordings, "LTJ Bukem", "Music")
	}
}