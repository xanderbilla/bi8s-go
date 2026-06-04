package main

// --- SUCCESS & ERROR ENVELOPES ---

type SuccessEnvelope struct {
	Success   bool            `json:"success"`
	Status    int             `json:"status"`
	Message   string          `json:"message"`
	Path      string          `json:"path,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Error     *APIErrorDetail `json:"error,omitempty"`
}

type APIErrorDetail struct {
	Type        string `json:"type"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	UserMessage string `json:"userMessage,omitempty"`
}

// --- RELATIONAL STRUCTS ---

type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Asset struct {
	Type string     `json:"type"`
	Keys []AssetKey `json:"keys"`
}

type AssetKey struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type ContentStats struct {
	TotalViews    int64   `json:"totalViews"`
	TotalLikes    int64   `json:"totalLikes"`
	AverageRating float64 `json:"averageRating"`
}

// --- CONTENT MODELS ---

type ContentListResponse struct {
	SuccessEnvelope
	Data PagedAdminContent `json:"data"`
}

type PagedAdminContent struct {
	Items  []ContentDetail `json:"items"`
	Cursor string          `json:"cursor"`
}

type ContentResponse struct {
	SuccessEnvelope
	Data ContentDetail `json:"data"`
}

type ContentDetail struct {
	ID               string       `json:"id"`
	Title            string       `json:"title"`
	Overview         string       `json:"overview"`
	ContentType      string       `json:"contentType"`
	OriginalLanguage string       `json:"originalLanguage"`
	ReleaseDate      string       `json:"releaseDate,omitempty"`
	FirstAirDate     string       `json:"firstAirDate,omitempty"`
	Adult            bool         `json:"adult"`
	ContentRating    string       `json:"contentRating,omitempty"`
	Runtime          int          `json:"runtime"`
	Status           string       `json:"status,omitempty"`
	Tagline          string       `json:"tagline,omitempty"`
	Visibility       string       `json:"visibility,omitempty"`
	OriginCountry    []string     `json:"originCountry,omitempty"`
	Genres           []EntityRef  `json:"genres,omitempty"`
	Casts            []EntityRef  `json:"casts,omitempty"`
	Tags             []EntityRef  `json:"tags,omitempty"`
	MoodTags         []EntityRef  `json:"moodTags,omitempty"`
	Studios          []EntityRef  `json:"studios,omitempty"`
	Assets           []Asset      `json:"assets,omitempty"`
	BackdropPath     string       `json:"backdropPath,omitempty"`
	PosterPath       string       `json:"posterPath,omitempty"`
	Stats            ContentStats `json:"stats,omitempty"`
}

func (c ContentDetail) EffectiveReleaseDate() string {
	if c.ReleaseDate != "" {
		return c.ReleaseDate
	}
	if c.ContentType == "TV" {
		return c.FirstAirDate
	}
	return ""
}

// --- PEOPLE MODELS ---

type PeopleListResponse struct {
	SuccessEnvelope
	Data PagedAdminPeople `json:"data"`
}

type PagedAdminPeople struct {
	Items  []PersonDetail `json:"items"`
	Cursor string         `json:"cursor"`
}

type PersonResponse struct {
	SuccessEnvelope
	Data PersonDetail `json:"data"`
}

type PersonDetail struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	LegalName    string       `json:"legalName,omitempty"`
	Roles        []string     `json:"roles"`
	StageName    string       `json:"stageName,omitempty"`
	Bio          string       `json:"bio,omitempty"`
	BirthDate    string       `json:"birthDate,omitempty"`
	BirthPlace   string       `json:"birthPlace,omitempty"`
	Nationality  string       `json:"nationality,omitempty"`
	Gender       string       `json:"gender"`
	Height       int          `json:"height"`
	Weight       int          `json:"weight"`
	Active       bool         `json:"active"`
	Verified     bool         `json:"verified"`
	DebutYear    int          `json:"debutYear,omitempty"`
	CareerStatus string       `json:"careerStatus"`
	ProfilePath  string       `json:"profilePath,omitempty"`
	BackdropPath string       `json:"backdropPath,omitempty"`
	Aliases      []string     `json:"aliases,omitempty"`
	Measurements Measurements `json:"measurements,omitempty"`
	Tags         []EntityRef  `json:"tags,omitempty"`
	Categories   []EntityRef  `json:"categories,omitempty"`
	Specialties  []EntityRef  `json:"specialties,omitempty"`
}

type Measurements struct {
	Bust      int    `json:"bust,omitempty"`
	Waist     int    `json:"waist,omitempty"`
	Hips      int    `json:"hips,omitempty"`
	Unit      string `json:"unit,omitempty"`
	BodyType  string `json:"bodyType,omitempty"`
	EyeColor  string `json:"eyeColor,omitempty"`
	HairColor string `json:"hairColor,omitempty"`
}

// --- ATTRIBUTES MODELS ---

type AttributeListResponse struct {
	SuccessEnvelope
	Data []AttributeDetail `json:"data"`
}

type AttributeResponse struct {
	SuccessEnvelope
	Data AttributeDetail `json:"data"`
}

type AttributeDetail struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	AttributeType []string `json:"attributeType"`
	Logo          string   `json:"logo,omitempty"`
	SVG           string   `json:"svg,omitempty"`
	ContentType   string   `json:"contentType"`
	Active        bool     `json:"active"`
}

// --- SYSTEM MODELS ---

type ReindexResponse struct {
	SuccessEnvelope
	Data ReindexResult `json:"data"`
}

type ReindexResult struct {
	People           int `json:"people"`
	Content          int `json:"content"`
	JoinTableEntries int `json:"joinTableEntries"`
}

type AssetUploadResponse struct {
	SuccessEnvelope
	Data AssetUploadResult `json:"data"`
}

type AssetUploadResult struct {
	ContentID     string   `json:"contentId"`
	AssetType     string   `json:"assetType"`
	UploadedCount int      `json:"uploadedCount"`
	Paths         []string `json:"paths"`
}

// --- ADMIN SEARCH MODELS ---

type AdminSearchResponse struct {
	SuccessEnvelope
	Data AdminSearchData `json:"data"`
}

type AdminSearchData struct {
	Query      string                       `json:"query"`
	Entity     string                       `json:"entity"`
	Sort       string                       `json:"sort"`
	Limit      int                          `json:"limit"`
	Cursor     int                          `json:"cursor"`
	Content    *AdminSearchContentSection   `json:"content,omitempty"`
	People     *AdminSearchPeopleSection    `json:"people,omitempty"`
	Attributes *AdminSearchAttributeSection `json:"attributes,omitempty"`
}

type AdminSearchContentSection struct {
	Items      []ContentDetail `json:"items"`
	Count      int             `json:"count"`
	Total      int             `json:"total"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type AdminSearchPeopleSection struct {
	Items      []PersonDetail `json:"items"`
	Count      int            `json:"count"`
	Total      int            `json:"total"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type AdminSearchAttributeSection struct {
	Items      []AttributeDetail `json:"items"`
	Count      int               `json:"count"`
	Total      int               `json:"total"`
	NextCursor string            `json:"nextCursor,omitempty"`
}
