// cmd/batch.go
package cmd

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "time"

    "github.com/cerberussg/tagger/pkg/enricher"
    "github.com/cerberussg/tagger/pkg/enricher/musicbrainz"
    "github.com/dhowden/tag"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var batchCmd = &cobra.Command{
    Use:   "batch <folder>",
    Short: "Process all AIFF files in a folder",
    Long: `Batch process all AIFF files in the specified folder, enriching
metadata with record label, release date, and genre information.

Examples:
  aiff-tagger batch ~/Music/DnB
  aiff-tagger batch ~/Downloads/new-releases --genre house --dry-run
  aiff-tagger batch . --verbose`,
    Args: cobra.ExactArgs(1),
    Run:  runBatch,
}

var (
    genreHint   string
    recursive   bool
    htmlReport  string
    enrichData  bool
)

func init() {
    rootCmd.AddCommand(batchCmd)

    batchCmd.Flags().StringVarP(&genreHint, "genre", "g", "", "genre hint for better API matching (dnb, house, breakbeat, etc.)")
    batchCmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "process subdirectories recursively")
    batchCmd.Flags().StringVar(&htmlReport, "html-report", "", "generate HTML report of edge cases (e.g., --html-report edge-cases.html)")
    batchCmd.Flags().BoolVar(&enrichData, "enrich", false, "enable metadata enrichment via API (respects --dry-run)")
}

func runBatch(cmd *cobra.Command, args []string) {
    folder := args[0]
    
    // Validate folder exists
    if !isValidDirectory(folder) {
        fmt.Printf("Error: Directory '%s' does not exist or is not accessible\n", folder)
        return
    }

    // Get absolute path
    absPath, err := filepath.Abs(folder)
    if err != nil {
        fmt.Printf("Error: Could not resolve path '%s': %v\n", folder, err)
        return
    }

    fmt.Printf("Processing folder: %s\n", absPath)
    if genreHint != "" {
        fmt.Printf("Genre hint: %s\n", genreHint)
    }
    if viper.GetBool("dry-run") {
        fmt.Println("DRY RUN: No files will be modified")
    }
    if enrichData {
        fmt.Println("ENRICHMENT: Enabled - will lookup missing metadata via MusicBrainz")
    }
    
    // Initialize enricher if needed
    var metadataEnricher *enricher.Enricher
    if enrichData {
        provider := musicbrainz.NewMusicBrainzProvider()
        defer provider.Close()
        
        config := &enricher.EnricherConfig{
            Strategy:       enricher.StrategyFirst,
            MinConfidence:  0.7,
            RequireLabel:   false,
            RequestTimeout: 30 * time.Second,
        }
        
        metadataEnricher = enricher.NewEnricher([]enricher.MetadataProvider{provider}, config)
        defer metadataEnricher.Close()
        
        fmt.Printf("Enricher initialized with strategy: %s\n", config.Strategy)
    }
    
    // Find audio files
    files, err := findAudioFiles(absPath, recursive, getSupportedExtensions())
    if err != nil {
        fmt.Printf("Error scanning directory: %v\n", err)
        return
    }

    if len(files) == 0 {
        fmt.Println("No supported audio files found in the specified directory")
        return
    }

    fmt.Printf("Found %d audio files\n\n", len(files))
    
    // Track what needs enrichment and edge cases
    var needsEnrichment int
    var hasLabel int
    var errorCount int
    var enrichmentSuccess int
    var enrichmentFailed int
    
    // Edge case tracking - store full paths instead of just filenames
    edgeCases := make(map[string][]string)
    
    // Context for API calls
    ctx := context.Background()
    if enrichData {
        // Add timeout for the entire batch process
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
        defer cancel()
    }
    
    // Process each file
    for i, file := range files {
        if viper.GetBool("verbose") {
            fmt.Printf("[%d/%d] %s\n", i+1, len(files), file)
        }
        
        status, edgeCase := processFileWithEdgeCase(file, metadataEnricher, ctx)
        switch status {
        case "needs_enrichment":
            needsEnrichment++
        case "has_label":
            hasLabel++
        case "error":
            errorCount++
        case "enriched":
            enrichmentSuccess++
        case "enrichment_failed":
            enrichmentFailed++
        }
        
        // Collect edge cases with full file paths
        if edgeCase != "" {
            edgeCases[edgeCase] = append(edgeCases[edgeCase], file)
        }
    }
    
    // Summary
    fmt.Printf("\n=== SUMMARY ===\n")
    fmt.Printf("Total files found: %d\n", len(files))
    fmt.Printf("Files with label info: %d\n", hasLabel)
    fmt.Printf("Files needing enrichment: %d\n", needsEnrichment)
    if errorCount > 0 {
        fmt.Printf("Files with read errors: %d\n", errorCount)
    }
    
    // Enrichment summary
    if enrichData {
        fmt.Printf("\n=== ENRICHMENT RESULTS ===\n")
        fmt.Printf("Successfully enriched: %d\n", enrichmentSuccess)
        fmt.Printf("Enrichment failed: %d\n", enrichmentFailed)
        if enrichmentSuccess > 0 {
            successRate := float64(enrichmentSuccess) / float64(enrichmentSuccess+enrichmentFailed) * 100
            fmt.Printf("Success rate: %.1f%%\n", successRate)
        }
    }
    
    // Edge cases summary
    totalEdgeCases := 0
    for _, files := range edgeCases {
        totalEdgeCases += len(files)
    }
    
    if totalEdgeCases > 0 {
        fmt.Printf("Files with parsing edge cases: %d\n", totalEdgeCases)
        
        fmt.Printf("\n=== EDGE CASES ===\n")
        for caseType, filePaths := range edgeCases {
            fmt.Printf("\n%s (%d files):\n", strings.ToUpper(strings.Replace(caseType, "_", " ", -1)), len(filePaths))
            for _, filePath := range filePaths {
                filename := filepath.Base(filePath)
                fmt.Printf("%s\n", filename)
            }
        }
    }
    
    // Generate HTML report if requested
    if htmlReport != "" && totalEdgeCases > 0 {
        err := generateHTMLReport(edgeCases, htmlReport)
        if err != nil {
            fmt.Printf("Error generating HTML report: %v\n", err)
        } else {
            fmt.Printf("\nHTML report generated: %s\n", htmlReport)
        }
    }
    
    if needsEnrichment > 0 {
        percentage := float64(needsEnrichment) / float64(len(files)) * 100
        fmt.Printf("\nRecommendation: %.1f%% of your collection could benefit from metadata enrichment\n", percentage)
    } else {
        fmt.Println("\nYour collection looks well-tagged! 🎉")
    }
}

func isValidDirectory(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    
    return info.IsDir()
}

// getSupportedExtensions returns the currently supported audio file extensions
func getSupportedExtensions() []string {
    return []string{".aiff", ".aif"} // TODO: Add .mp3, .flac, .wav when implemented
}

// findAudioFiles finds all supported audio files in a directory
func findAudioFiles(root string, recursive bool, extensions []string) ([]string, error) {
    var files []string
    
    // Convert extensions to lowercase for comparison
    supportedExts := make(map[string]bool)
    for _, ext := range extensions {
        supportedExts[strings.ToLower(ext)] = true
    }
    
    if recursive {
        err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
            if err != nil {
                return err
            }
            if !d.IsDir() {
                // Skip AppleDouble files (._filename)
                if strings.HasPrefix(d.Name(), "._") {
                    return nil
                }
                
                ext := strings.ToLower(filepath.Ext(path))
                if supportedExts[ext] {
                    files = append(files, path)
                }
            }
            return nil
        })
        return files, err
    } else {
        // Non-recursive: check immediate directory for all supported extensions
        for _, extension := range extensions {
            pattern := filepath.Join(root, "*"+extension)
            matches, err := filepath.Glob(pattern)
            if err != nil {
                continue // Skip this extension if glob fails
            }
            
            // Filter out AppleDouble files
            for _, match := range matches {
                filename := filepath.Base(match)
                if !strings.HasPrefix(filename, "._") {
                    files = append(files, match)
                }
            }
        }
        return files, nil
    }
}

// Legacy function for backward compatibility - can be removed later
func findAIFFFiles(root string, recursive bool) ([]string, error) {
    return findAudioFiles(root, recursive, []string{".aiff", ".aif"})
}

func processFileWithEdgeCase(filePath string, metadataEnricher *enricher.Enricher, ctx context.Context) (status, edgeCase string) {
    if viper.GetBool("verbose") {
        fmt.Printf("  Reading metadata: %s\n", filePath)
    }
    
    // Try to read metadata using dhowden/tag
    file, err := os.Open(filePath)
    if err != nil {
        if viper.GetBool("verbose") {
            fmt.Printf("  ❌ Error opening file: %v\n", err)
        }
        return "error", ""
    }
    defer file.Close()
    
    metadata, err := tag.ReadFrom(file)
    
    var title, artist, album, genre, labelInfo string
    var hasLabel bool
    var year int
    var parseEdgeCase string
    
    if err != nil {
        // No embedded tags - try filename parsing
        if viper.GetBool("verbose") {
            fmt.Printf("  ⚠️  No embedded tags found - parsing filename\n")
        }
        
        parsedArtist, parsedTitle, edgeType := parseFilenameWithEdgeCase(filePath)
        
        if viper.GetBool("verbose") {
            fmt.Printf("  🔧 parseFilenameWithEdgeCase returned: artist='%s', title='%s'\n", parsedArtist, parsedTitle)
        }
        
        artist = parsedArtist
        title = parsedTitle
        parseEdgeCase = edgeType
        
    } else {
        // Has embedded tags - use those
        title = strings.TrimSpace(metadata.Title())
        artist = strings.TrimSpace(metadata.Artist())
        album = strings.TrimSpace(metadata.Album())
        genre = strings.TrimSpace(metadata.Genre())
        year = metadata.Year()
        
        // Check for label info in raw tags
        if rawTags := metadata.Raw(); rawTags != nil {
            if pub, ok := rawTags["TPUB"]; ok {
                labelInfo = fmt.Sprintf("%v", pub)
                hasLabel = labelInfo != ""
            }
            if txxx, ok := rawTags["TXXX"]; ok {
                labelInfo = fmt.Sprintf("%v", txxx)
                hasLabel = labelInfo != ""
            }
        }
    }
    
    hasBasicInfo := title != "" && artist != ""
    
    if viper.GetBool("verbose") {
        fmt.Printf("  Artist: %s\n", artist)
        fmt.Printf("  Title: %s\n", title)
        if album != "" {
            fmt.Printf("  Album: %s\n", album)
        }
        if labelInfo != "" {
            fmt.Printf("  Label: %s\n", labelInfo)
        }
        if year > 0 {
            fmt.Printf("  Year: %d\n", year)
        }
        if genre != "" {
            fmt.Printf("  Genre: %s\n", genre)
        }
        
        if !hasBasicInfo {
            filename := filepath.Base(filePath)
            fmt.Printf("  💡 Filename: %s\n", filename)
            fmt.Printf("  ⚠️  Could not parse artist/title from filename\n")
        }
    }
    
    if !hasBasicInfo {
        if viper.GetBool("verbose") {
            fmt.Printf("  📝 Unable to extract basic info - needs manual review\n")
        }
        return "needs_enrichment", parseEdgeCase
    }
    
    if hasLabel {
        if viper.GetBool("verbose") {
            fmt.Printf("  ✅ Has label info\n")
        }
        return "has_label", parseEdgeCase
    } else {
        // Try enrichment if enabled and we have basic info
        if metadataEnricher != nil && hasBasicInfo {
            if viper.GetBool("verbose") {
                fmt.Printf("  🔍 Attempting enrichment for: %s - %s\n", artist, title)
            }
            
            enrichedData, err := metadataEnricher.Lookup(ctx, artist, title)
            if err != nil {
                if viper.GetBool("verbose") {
                    fmt.Printf("  ❌ Enrichment failed: %v\n", err)
                }
                return "enrichment_failed", parseEdgeCase
            }
            
            if enrichedData != nil {
                if viper.GetBool("verbose") {
                    fmt.Printf("  🎉 Enrichment successful!\n")
                    fmt.Printf("    Label: %s\n", enrichedData.Label)
                    fmt.Printf("    Release Date: %s\n", enrichedData.ReleaseDate)
                    fmt.Printf("    Confidence: %.2f\n", enrichedData.Confidence)
                    if viper.GetBool("dry-run") {
                        fmt.Printf("    📝 Would write metadata (dry-run mode)\n")
                    } else {
                        fmt.Printf("    📝 Writing metadata to file\n")
                        // TODO: Implement actual metadata writing here
                    }
                }
                return "enriched", parseEdgeCase
            }
        }
        
        if viper.GetBool("verbose") {
            fmt.Printf("  📝 Ready for label enrichment via API\n")
        }
        return "needs_enrichment", parseEdgeCase
    }
}

// parseFilenameWithEdgeCase attempts to extract artist and title from filename
// Also returns edge case type if encountered
func parseFilenameWithEdgeCase(filePath string) (artist, title, edgeCase string) {
    filename := filepath.Base(filePath)
    
    // Remove file extension
    name := strings.TrimSuffix(filename, filepath.Ext(filename))
    
    // Clean up common prefixes first (track numbers, etc.)
    name = cleanTrackPrefix(name)
    
    // Count hyphens to determine parsing strategy
    hyphenCount := strings.Count(name, "-")
    
    if viper.GetBool("verbose") {
        fmt.Printf("  🔍 Parsing filename: %s (hyphens: %d)\n", name, hyphenCount)
    }
    
    switch hyphenCount {
    case 0:
        // No hyphens - can't reliably parse
        fmt.Printf("  🚨 SWITCH: case 0 - calling handleEdgeCase\n")
        artist, title = handleEdgeCase(name, "no_hyphens")
        edgeCase = "no_hyphens"
        
    case 1:
        // Artist - Title
        if viper.GetBool("verbose") {
            fmt.Printf("  🎯 Using parseOneHyphen for 1 hyphen case\n")
        }
        fmt.Printf("  🚨 SWITCH: case 1 - about to call parseOneHyphen\n")
        artist, title = parseOneHyphen(name)
        fmt.Printf("  🚨 SWITCH: case 1 - parseOneHyphen returned: artist='%s', title='%s'\n", artist, title)
        edgeCase = ""
        
    case 2:
        // Artist - Album - Title
        if viper.GetBool("verbose") {
            fmt.Printf("  🎯 Using parseTwoHyphens for 2 hyphen case\n")
        }
        fmt.Printf("  🚨 SWITCH: case 2 - calling parseTwoHyphens\n")
        artist, title = parseTwoHyphens(name)
        edgeCase = ""
        
    case 3:
        // Edge case - needs manual review or special handling
        artist, title = handleEdgeCase(name, "three_hyphens")
        edgeCase = "three_hyphens"
        
    case 4:
        // Artist/Part - Album/Part - Title
        // Replace 1st and 3rd hyphens with slashes
        artist, title = parseFourHyphens(name)
        edgeCase = ""
        
    default:
        // 5+ hyphens - likely very complex, needs edge case handling
        artist, title = handleEdgeCase(name, "many_hyphens")
        edgeCase = "many_hyphens"
    }
    
    if viper.GetBool("verbose") {
        fmt.Printf("  🎯 Final parsing result: artist='%s', title='%s', edgeCase='%s'\n", artist, title, edgeCase)
    }
    
    return artist, title, edgeCase
}

// parseFilename attempts to extract artist and title from filename
// Handles complex hyphen patterns common in D&B collections
func parseFilename(filePath string) (artist, title string) {
    artist, title, _ = parseFilenameWithEdgeCase(filePath)
    return artist, title
}

func parseOneHyphen(name string) (artist, title string) {
    parts := strings.SplitN(name, "-", 2)
    if len(parts) == 2 {
        artist = strings.TrimSpace(parts[0])
        title = strings.TrimSpace(parts[1])
        return artist, title
    }
    return "", ""
}

func parseTwoHyphens(name string) (artist, title string) {
    parts := strings.SplitN(name, "-", 3)
    if len(parts) == 3 {
        artist = strings.TrimSpace(parts[0])
        // Skip album (parts[1]) for now - we just want artist/title
        title = strings.TrimSpace(parts[2])
        return artist, title
    }
    return "", ""
}

func parseFourHyphens(name string) (artist, title string) {
    parts := strings.SplitN(name, "-", 5)
    if len(parts) == 5 {
        // Reconstruct artist: parts[0] / parts[1]
        artistPart1 := strings.TrimSpace(parts[0])
        artistPart2 := strings.TrimSpace(parts[1])
        artist = artistPart1 + "/" + artistPart2
        
        // Skip album parts[2]/parts[3] 
        title = strings.TrimSpace(parts[4])
        
        if viper.GetBool("verbose") {
            albumPart1 := strings.TrimSpace(parts[2])
            albumPart2 := strings.TrimSpace(parts[3])
            album := albumPart1 + "/" + albumPart2
            fmt.Printf("  📀 Detected album: %s\n", album)
        }
        
        return artist, title
    }
    return "", ""
}

func handleEdgeCase(name string, caseType string) (artist, title string) {
    if viper.GetBool("verbose") {
        fmt.Printf("  ⚠️  Edge case (%s): %s\n", caseType, name)
        fmt.Printf("  💡 Consider manual review or custom parsing rule\n")
    }
    
    // For edge cases, try some fallback strategies
    switch caseType {
    case "no_hyphens":
        // Maybe it's "Artist Title" with spaces?
        return trySpaceSeparated(name)
        
    case "three_hyphens":
        // Try treating as Artist - Album - Extra - Title
        return tryThreeHyphenFallback(name)
        
    case "many_hyphens":
        // Try to find the most likely artist-title split
        return tryManyHyphenFallback(name)
        
    default:
        return "", ""
    }
}

func trySpaceSeparated(name string) (artist, title string) {
    // Look for patterns like "ArtistName SongTitle"
    // This is tricky without more context, so be conservative
    words := strings.Fields(name)
    if len(words) >= 2 {
        // Simple heuristic: first word(s) as artist, rest as title
        // But this is unreliable, so only do it if we're confident
        if len(words) == 2 {
            return words[0], words[1]
        }
    }
    return "", ""
}

func tryThreeHyphenFallback(name string) (artist, title string) {
    parts := strings.SplitN(name, "-", 4)
    if len(parts) == 4 {
        // Try different interpretations:
        // 1. Artist - Album Part 1 - Album Part 2 - Title
        artist = strings.TrimSpace(parts[0])
        title = strings.TrimSpace(parts[3])
        
        if viper.GetBool("verbose") {
            fmt.Printf("  🤔 Guessing: Artist=%s, Title=%s\n", artist, title)
        }
        
        return artist, title
    }
    return "", ""
}

func tryManyHyphenFallback(name string) (artist, title string) {
    // For 5+ hyphens, try to find the longest reasonable artist name
    // and assume everything after the last 1-2 hyphens is the title
    parts := strings.Split(name, "-")
    if len(parts) >= 3 {
        // Take first part as artist, last part as title
        artist = strings.TrimSpace(parts[0])
        title = strings.TrimSpace(parts[len(parts)-1])
        
        if viper.GetBool("verbose") {
            fmt.Printf("  🎯 Best guess: Artist=%s, Title=%s\n", artist, title)
        }
        
        return artist, title
    }
    return "", ""
}

// cleanTrackPrefix removes common prefixes from the entire filename
func cleanTrackPrefix(name string) string {
    // Remove track numbers like "01 ", "1. ", "A1 ", "B2 ", etc.
    patterns := []string{
        `^\d+\.?\s+`,        // "01 " or "1. "
        `^\d+\s*-\s*`,       // "01-" or "1 - "
        `^[A-Z]\d+\s+`,      // "A1 " or "B2 "
        `^[A-Z]\d+\s*-\s*`,  // "A1-" or "B2 - "
    }
    
    for _, pattern := range patterns {
        re := regexp.MustCompile(pattern)
        name = re.ReplaceAllString(name, "")
    }
    
    return strings.TrimSpace(name)
}

// generateHTMLReport creates an HTML file showing edge cases with links to file locations
func generateHTMLReport(edgeCases map[string][]string, outputPath string) error {
    file, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // HTML header
    html := `<!DOCTYPE html>
<html>
<head>
    <title>Library Edge Cases</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        h2 { color: #666; margin-top: 30px; }
        table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f2f2f2; }
        tr:nth-child(even) { background-color: #f9f9f9; }
        .description { background-color: #f0f8ff; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .path { font-family: monospace; font-size: 0.9em; color: #666; word-break: break-all; cursor: pointer; }
        .path:hover { background-color: #f0f0f0; }
        .copy-hint { font-size: 0.8em; color: #888; font-style: italic; }
    </style>
    <script>
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(function() {
                alert('Path copied to clipboard!');
            }).catch(function() {
                // Fallback for older browsers
                prompt('Copy this path:', text);
            });
        }
    </script>
</head>
<body>
    <h1>Library Edge Cases</h1>
    <div class="description">
        <p>These files have naming patterns that couldn't be automatically parsed for artist and title extraction. 
        They may need manual review or custom parsing rules.</p>
        <p><strong>💡 Tip:</strong> Click on any path to copy it to your clipboard, then use ⌘+Shift+G in Finder to navigate there.</p>
        <p><strong>Edge Case Types:</strong></p>
        <ul>
            <li><strong>No Hyphens:</strong> Files without hyphen separators</li>
            <li><strong>Three Hyphens:</strong> Ambiguous patterns requiring manual review</li>
            <li><strong>Many Hyphens:</strong> Complex patterns with 5+ hyphens</li>
        </ul>
    </div>
`
    
    // Add each edge case category
    for caseType, filePaths := range edgeCases {
        categoryTitle := strings.ToUpper(strings.Replace(caseType, "_", " ", -1))
        html += fmt.Sprintf(`
    <h2>%s (%d files)</h2>
    <table>
        <thead>
            <tr>
                <th>File Name</th>
                <th>Path (Click to Copy)</th>
            </tr>
        </thead>
        <tbody>
`, categoryTitle, len(filePaths))
        
        for _, filePath := range filePaths {
            filename := filepath.Base(filePath)
            directory := filepath.Dir(filePath)
            
            html += fmt.Sprintf(`            <tr>
                <td>%s</td>
                <td class="path" onclick="copyToClipboard('%s')" title="Click to copy path">%s<br><span class="copy-hint">📋 Click to copy</span></td>
            </tr>
`, filename, directory, directory)
        }
        
        html += `        </tbody>
    </table>
`
    }
    
    // HTML footer
    html += `
</body>
</html>`
    
    _, err = file.WriteString(html)
    return err
}