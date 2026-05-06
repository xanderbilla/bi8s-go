package model

type SearchContentPage struct {
	Results  []MoviePublicList `json:"results"`
	Count    int               `json:"count"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type SearchPeoplePage struct {
	Results  []SearchPersonResult `json:"results"`
	Count    int                  `json:"count"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

type SearchPersonResult struct {
	ID           string       `json:"id"`
	ContentType  ContentType  `json:"contentType"`
	Name         string       `json:"name"`
	Roles        []EntityType `json:"roles"`
	StageName    string       `json:"stageName,omitempty"`
	Bio          string       `json:"bio,omitempty"`
	Gender       Gender       `json:"gender"`
	Verified     bool         `json:"verified"`
	ProfilePath  string       `json:"profilePath,omitempty"`
	BackdropPath string       `json:"backdropPath,omitempty"`
	Categories   []EntityRef  `json:"categories,omitempty"`
	Specialties  []EntityRef  `json:"specialties,omitempty"`
}

type SearchResponse struct {
	Content  SearchContentPage `json:"content"`
	People   SearchPeoplePage  `json:"people"`
	Warnings []SearchWarning   `json:"warnings,omitempty"`
}

type SearchWarning struct {
	Scope   string `json:"scope"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
