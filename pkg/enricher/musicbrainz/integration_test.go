// pkg/enricher/musicbrainz/integration_test.go
// +build integration

package musicbrainz

import (
	"context"
	"testing"
	"time"

	"github.com/cerberussg/tagger/pkg/enricher"
)

// Integration tests that hit the real MusicBrainz API
// Run with: go test -tags=integration

func TestMusicBrainzProvider_Integration_RealAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	provider := NewMusicBrainzProvider()
	defer provider.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	testCases := []struct {
		name           string
		artist         string
		title          string
		expectSuccess  bool
		expectedLabel  string
	}{
		{
			name:          "LTJ Bukem - Music",
			artist:        "LTJ Bukem",
			title:         "Music",
			expectSuccess: true,
			expectedLabel: "Good Looking Records", // We expect this label
		},
		{
			name:          "Goldie - Inner City Life",
			artist:        "Goldie",
			title:         "Inner City Life",
			expectSuccess: true,
			expectedLabel: "Metalheadz", // We expect this label
		},
		{
			name:          "Nonexistent Artist - Fake Song",
			artist:        "ThisArtistDoesNotExist12345",
			title:         "ThisSongDoesNotExist12345",
			expectSuccess: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.Lookup(ctx, tc.artist, tc.title)
			
			if tc.expectSuccess {
				if err != nil {
					t.Fatalf("Expected success but got error: %v", err)
				}
				
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
				
				// Validate basic fields
				if result.Artist == "" {
					t.Error("Expected artist to be populated")
				}
				
				if result.Title == "" {
					t.Error("Expected title to be populated")
				}
				
				if result.ProviderName != "MusicBrainz" {
					t.Errorf("Expected provider name 'MusicBrainz', got '%s'", result.ProviderName)
				}
				
				if result.Confidence <= 0 || result.Confidence > 1 {
					t.Errorf("Expected confidence between 0 and 1, got %f", result.Confidence)
				}
				
				// Check if we got the expected label (if specified)
				if tc.expectedLabel != "" && result.Label != "" {
					if result.Label != tc.expectedLabel {
						t.Logf("Note: Expected label '%s', got '%s' - this might be due to MusicBrainz data changes", 
							tc.expectedLabel, result.Label)
					}
				}
				
				t.Logf("Success: Found %s - %s (Label: %s, Confidence: %.2f)", 
					result.Artist, result.Title, result.Label, result.Confidence)
				
			} else {
				if err == nil {
					t.Error("Expected error for nonexistent track but got success")
				}
				
				if err != enricher.ErrNotFound {
					t.Logf("Got error: %v (expected ErrNotFound but other errors are acceptable)", err)
				}
			}
		})
	}
}

func TestMusicBrainzProvider_Integration_WithHints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	provider := NewMusicBrainzProvider()
	defer provider.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Test with album hint
	req := &enricher.SearchRequest{
		Artist:                "Roni Size",
		Title:                 "Brown Paper Bag",
		Album:                 "New Forms",
		PreferOriginalRelease: true,
		MaxResults:           5,
	}
	
	result, err := provider.LookupWithHints(ctx, req)
	
	if err != nil {
		t.Fatalf("Lookup with hints failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result but got nil")
	}
	
	// Should find the track with better confidence due to album hint
	if result.Confidence < 0.5 {
		t.Errorf("Expected higher confidence with album hint, got %f", result.Confidence)
	}
	
	t.Logf("Found with hints: %s - %s from %s (Label: %s, Confidence: %.2f)", 
		result.Artist, result.Title, result.Album, result.Label, result.Confidence)
}

func TestMusicBrainzProvider_Integration_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	provider := NewMusicBrainzProvider()
	defer provider.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Make two rapid requests to test rate limiting
	start := time.Now()
	
	_, err1 := provider.Lookup(ctx, "LTJ Bukem", "Music")
	if err1 != nil && err1 != enricher.ErrNotFound {
		t.Fatalf("First request failed: %v", err1)
	}
	
	_, err2 := provider.Lookup(ctx, "Goldie", "Inner City Life")
	if err2 != nil && err2 != enricher.ErrNotFound {
		t.Fatalf("Second request failed: %v", err2)
	}
	
	elapsed := time.Since(start)
	
	// Should take at least 1 second due to rate limiting
	if elapsed < time.Second {
		t.Errorf("Rate limiting not working: both requests completed in %v", elapsed)
	}
	
	t.Logf("Rate limiting working: two requests took %v", elapsed)
}

func TestMusicBrainzProvider_Integration_ErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	provider := NewMusicBrainzProvider()
	defer provider.Close()
	
	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err := provider.Lookup(cancelledCtx, "LTJ Bukem", "Music")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	
	// Test with very short timeout
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer shortCancel()
	
	_, err = provider.Lookup(shortCtx, "LTJ Bukem", "Music")
	if err == nil {
		t.Error("Expected timeout error but got success")
	}
	
	t.Logf("Timeout test completed with error: %v", err)
}

// Benchmark against real API (use sparingly to respect rate limits)
func BenchmarkMusicBrainzProvider_Integration_RealLookup(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping integration benchmarks in short mode")
	}
	
	provider := NewMusicBrainzProvider()
	defer provider.Close()
	
	ctx := context.Background()
	
	// Only run a few iterations to respect MusicBrainz rate limits
	if b.N > 5 {
		b.N = 5
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := provider.Lookup(ctx, "LTJ Bukem", "Music")
		if err != nil && err != enricher.ErrNotFound {
			b.Fatalf("Lookup failed: %v", err)
		}
		
		// Respect rate limiting
		time.Sleep(1100 * time.Millisecond)
	}
}