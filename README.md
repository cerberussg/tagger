# Tagger

A CLI tool for analyzing and enriching audio metadata, currently supporting AIFF files with plans to expand to additional formats. Specifically designed for DJs to maintain collections but anyone is welcome.

## Problem Solved

Many audio files, especially AIFF vinyl rips and older digital collections, lack proper embedded metadata. This creates several issues for DJs and music collectors:

- **Missing record label information** - Critical for genre identification and collection organization
- **No release dates** - Important for chronological organization and historical context  
- **Inconsistent metadata** - Makes it difficult to search and filter in DJ software like RekordBox
- **Complex filename patterns** - Files often use intricate naming conventions that are hard to parse automatically

Tagger solves these problems by:
- **Intelligently parsing filenames** to extract artist and track information
- **Identifying files that need metadata enrichment** vs. those already properly tagged
- **Handling complex naming patterns** with multiple hyphens and catalog numbers
- **Generating detailed reports** for edge cases that need manual attention
- **Preparing for API integration** to automatically fetch missing label and release data

## Features

- 🔍 **Smart filename parsing** - Handles complex hyphen patterns common in D&B collections
- 📊 **Collection analysis** - Shows percentage of files needing enrichment
- 🚫 **AppleDouble filtering** - Automatically excludes `._` system files
- 📋 **Edge case reporting** - HTML reports with clickable paths for problematic files
- 🏷️ **Metadata detection** - Checks existing embedded ID3 tags
- 🎛️ **Configurable settings** - Persistent configuration management
- 🔄 **Recursive scanning** - Process entire directory trees
- 👀 **Dry-run mode** - Preview changes without modifying files

## Installation

### Prerequisites
- Go 1.21 or later
- macOS, Linux, or Windows

### Build from Source

```bash
# Clone the repository (when available)
git clone https://github.com/yourusername/tagger
cd tagger

# Initialize Go module and install dependencies
go mod init tagger
go mod tidy

# Build the binary
go build -o tagger

# Make it executable (Unix/macOS)
chmod +x tagger

# Optional: Install globally
sudo mv tagger /usr/local/bin/
```

## Usage

### Basic Commands

#### Analyze a Collection
```bash
# Basic analysis with dry-run (safe, no changes made)
./tagger batch ~/Music/DnB --dry-run

# Verbose output to see detailed file information
./tagger batch ~/Music/DnB --dry-run --verbose

# Process only immediate directory (non-recursive)
./tagger batch ~/Music/DnB --dry-run --recursive=false
```

#### Generate Edge Case Reports
```bash
# Create HTML report for files that couldn't be parsed
./tagger batch ~/Music/DnB --dry-run --html-report edge-cases.html
```

#### Genre-Specific Processing
```bash
# Provide genre hints for better future API matching
./tagger batch ~/Music/House --dry-run --genre house
./tagger batch ~/Music/Breakbeats --dry-run --genre breakbeat
```

### Configuration Management

```bash
# View current configuration
./tagger config show

# Set API rate limiting (for future API integration)
./tagger config set api.musicbrainz.rate_limit 15

# Set directories to watch (for future daemon mode)
./tagger config set watch_dirs "~/Music/DnB,~/Downloads"

# View specific setting
./tagger config show api.musicbrainz.rate_limit
```

### Command Reference

#### `batch` Command
Process all AIFF files in a specified directory.

**Usage:** `tagger batch <folder> [flags]`

**Flags:**
- `--dry-run` - Show what would be done without making changes (recommended)
- `--verbose` - Show detailed information about each file processed
- `--recursive, -r` - Process subdirectories recursively (default: true)
- `--genre, -g` - Genre hint for better API matching (dnb, house, breakbeat, etc.)
- `--html-report` - Generate HTML report of edge cases (e.g., `--html-report report.html`)
- `--config` - Specify custom config file path

**Global Flags:**
- `--help, -h` - Show help information
- `--version` - Show version information

#### `config` Command
Manage configuration settings.

**Subcommands:**
- `show [key]` - Display configuration values
- `set <key> <value>` - Set a configuration value

**Available Configuration Keys:**
- `api.musicbrainz.rate_limit` - API calls per minute (default: 10)
- `api.musicbrainz.user_agent` - User agent for API requests
- `processing.concurrent_workers` - Number of parallel workers (default: 3)
- `cache.ttl_hours` - Cache TTL in hours (default: 168)
- `watch_dirs` - Comma-separated list of directories to watch

## Examples

### Typical Workflow

1. **Initial collection analysis:**
```bash
./tagger batch ~/Music/DnB --dry-run --verbose
```

2. **Generate edge case report:**
```bash
./tagger batch ~/Music/DnB --dry-run --html-report dnb-edge-cases.html
```

3. **Review the HTML report** in your browser and manually fix problematic filenames

4. **Configure settings for your collection:**
```bash
./tagger config set watch_dirs "~/Music/DnB,~/Music/Liquid,~/Music/Neurofunk"
./tagger config set api.musicbrainz.rate_limit 12
```

### Understanding the Output

```bash
$ ./tagger batch ~/Music/DnB --dry-run

Processing folder: /Users/you/Music/DnB
DRY RUN: No files will be modified
Found 150 AIFF files

=== SUMMARY ===
Total files found: 150
Files with label info: 12
Files needing enrichment: 138
Files with parsing edge cases: 8

MANY HYPHENS (5 files):
Alex Reece - MDZ03 - No Smoke Without Fire - 09 Pulp Fiction -Lemon D Remix-.aiff
Asylum - 25 Years of Metalheadz - Part 2 - 01 Da Base II Dark -Remastered-.aiff

THREE HYPHENS (3 files):
Artist - Something - Else - Title.aiff

Recommendation: 92.0% of your collection could benefit from metadata enrichment
```

### Filename Patterns Supported

The tool intelligently handles various music naming conventions:

- **1 Hyphen:** `Artist - Title.aiff`
- **2 Hyphens:** `Artist - Album - Title.aiff`
- **4 Hyphens:** `Artist/Part - Album/Part - Title.aiff` (reconstructs with slashes)
- **Track prefixes:** `01 Artist - Title.aiff`, `A1 Artist - Title.aiff`
- **Edge cases:** 3+ hyphens flagged for manual review

## HTML Edge Case Reports

When using `--html-report`, you'll get a styled HTML file with:

- **Categorized edge cases** by complexity type
- **Clickable paths** that copy directory locations to clipboard
- **File counts** for each category
- **Clean, printable format** for manual review

Use ⌘+Shift+G (macOS) or Ctrl+L (Linux) in your file manager to navigate to copied paths.

## Configuration File

Settings are stored in `~/.tagger/config.yaml`:

```yaml
api:
  musicbrainz:
    rate_limit: 10
    user_agent: "tagger/0.1.0"
processing:
  concurrent_workers: 3
cache:
  ttl_hours: 168
watch_dirs:
  - "~/Music/DnB"
  - "~/Downloads"
```

## Roadmap

- 🎵 **MusicBrainz API integration** - Automatic label and release date fetching
- 📝 **Metadata writing** - Write enriched data back to audio files
- 🔄 **Daemon mode** - Background processing of new files
- 🎚️ **Additional APIs** - Discogs, Last.fm integration for better coverage
- 📊 **Collection statistics** - Detailed analytics about your music library
- 🎧 **Format expansion** - MP3, FLAC, WAV, and other audio format support

## Contributing

This tool is designed specifically for music collections but welcomes contributions for:
- Additional filename pattern support
- New music databases integration
- Performance improvements
- Cross-platform compatibility enhancements

## License

MIT License - see LICENSE file for details.

## Support

For issues, feature requests, or questions about metadata patterns, please open an issue on GitHub.