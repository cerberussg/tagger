// pkg/enricher/musicbrainz/types.go

package musicbrainz

// RecordingSearchResult represents the response from MusicBrainz recording search
type RecordingSearchResult struct {
	Created    string      `json:"created"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
	Recordings []Recording `json:"recordings"`
}

// Recording represents a MusicBrainz recording (song/track)
type Recording struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Length       int            `json:"length,omitempty"`
	Score        int            `json:"score"` // Search relevance score
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Releases     []Release      `json:"releases,omitempty"`
}

// RecordingDetail represents detailed recording info with releases
type RecordingDetail struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Length       int            `json:"length,omitempty"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Releases     []Release      `json:"releases"`
}

// ArtistCredit represents artist credit information
type ArtistCredit struct {
	Name       string `json:"name"`
	Joinphrase string `json:"joinphrase,omitempty"`
	Artist     Artist `json:"artist"`
}

// Artist represents a MusicBrainz artist
type Artist struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name"`
	Disambiguation string   `json:"disambiguation,omitempty"`
	Aliases        []Alias  `json:"aliases,omitempty"`
}

// Alias represents an artist alias
type Alias struct {
	Name       string `json:"name"`
	SortName   string `json:"sort-name"`
	Type       string `json:"type,omitempty"`
	Primary    bool   `json:"primary,omitempty"`
	BeginDate  string `json:"begin-date,omitempty"`
	EndDate    string `json:"end-date,omitempty"`
}

// Release represents a MusicBrainz release (album/single)
type Release struct {
	ID                string       `json:"id"`
	Title             string       `json:"title"`
	Status            string       `json:"status,omitempty"`
	StatusID          string       `json:"status-id,omitempty"`
	Packaging         string       `json:"packaging,omitempty"`
	TextRepresentation TextRep     `json:"text-representation,omitempty"`
	ArtistCredit      []ArtistCredit `json:"artist-credit"`
	ReleaseGroup      ReleaseGroup `json:"release-group"`
	Date              string       `json:"date,omitempty"`
	Country           string       `json:"country,omitempty"`
	ReleaseEvents     []ReleaseEvent `json:"release-events,omitempty"`
	LabelInfo         []LabelInfo  `json:"label-info,omitempty"`
	TrackCount        int          `json:"track-count,omitempty"`
	Media             []Media      `json:"media,omitempty"`
}

// TextRep represents text representation info
type TextRep struct {
	Language string `json:"language,omitempty"`
	Script   string `json:"script,omitempty"`
}

// ReleaseGroup represents a MusicBrainz release group
type ReleaseGroup struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	PrimaryType        string   `json:"primary-type,omitempty"`
	PrimaryTypeID      string   `json:"primary-type-id,omitempty"`
	SecondaryTypes     []string `json:"secondary-types,omitempty"`
	SecondaryTypeIDs   []string `json:"secondary-type-ids,omitempty"`
	FirstReleaseDate   string   `json:"first-release-date,omitempty"`
	Disambiguation     string   `json:"disambiguation,omitempty"`
}

// ReleaseEvent represents a release event (date/country)
type ReleaseEvent struct {
	Date string `json:"date,omitempty"`
	Area Area   `json:"area,omitempty"`
}

// Area represents a geographical area
type Area struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SortName      string   `json:"sort-name"`
	ISO31661Codes []string `json:"iso-3166-1-codes,omitempty"`
}

// LabelInfo represents label information for a release
type LabelInfo struct {
	CatalogNumber string `json:"catalog-number,omitempty"`
	Label         Label  `json:"label"`
}

// Label represents a MusicBrainz label (record label)
type Label struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name,omitempty"`
	LabelCode      int    `json:"label-code,omitempty"`
	Type           string `json:"type,omitempty"`
	TypeID         string `json:"type-id,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
	Country        string `json:"country,omitempty"`
	LifeSpan       LifeSpan `json:"life-span,omitempty"`
}

// LifeSpan represents the active period of an entity
type LifeSpan struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
	Ended bool   `json:"ended,omitempty"`
}

// Media represents media (CD, vinyl, etc.) in a release
type Media struct {
	Title      string  `json:"title,omitempty"`
	Format     string  `json:"format,omitempty"`
	FormatID   string  `json:"format-id,omitempty"`
	Position   int     `json:"position"`
	TrackCount int     `json:"track-count"`
	Tracks     []Track `json:"tracks,omitempty"`
}

// Track represents an individual track on media
type Track struct {
	ID           string         `json:"id"`
	Position     int            `json:"position"`
	Title        string         `json:"title"`
	Length       int            `json:"length,omitempty"`
	Number       string         `json:"number"`
	ArtistCredit []ArtistCredit `json:"artist-credit,omitempty"`
	Recording    Recording      `json:"recording,omitempty"`
}

// Genre represents a MusicBrainz genre/tag
type Genre struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation,omitempty"`
}

// Tag represents a user-generated tag
type Tag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

// Coordinates represents geographical coordinates
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Error represents a MusicBrainz API error response
type Error struct {
	Error   string `json:"error"`
	Help    string `json:"help,omitempty"`
}

// SearchHint contains additional search parameters for advanced queries
type SearchHint struct {
	Artist      string
	Title       string
	Album       string
	Date        string
	Country     string
	Label       string
	CatalogNumber string
	TrackNumber int
	Duration    int // in milliseconds
}