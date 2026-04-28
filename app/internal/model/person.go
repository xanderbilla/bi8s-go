package model

type EntityType string

const (
	EntityTypePerformer      EntityType = "PERFORMER"
	EntityTypeContentCreator EntityType = "CONTENT_CREATOR"
)

type Gender string

const (
	GenderMale   Gender = "Male"
	GenderFemale Gender = "Female"
	GenderTrans  Gender = "Trans"
)

type CareerStatus string

const (
	CareerStatusActive  CareerStatus = "Active"
	CareerStatusRetired CareerStatus = "Retired"
	CareerStatusHiatus  CareerStatus = "Hiatus"
)

type Measurements struct {
	Bust      int    `json:"bust,omitempty" dynamodbav:"bust,omitempty"`
	Waist     int    `json:"waist,omitempty" dynamodbav:"waist,omitempty"`
	Hips      int    `json:"hips,omitempty" dynamodbav:"hips,omitempty"`
	Unit      string `json:"unit,omitempty" dynamodbav:"unit,omitempty" validate:"omitempty,oneof=inches cm"`
	BodyType  string `json:"bodyType,omitempty" dynamodbav:"bodyType,omitempty"`
	EyeColor  string `json:"eyeColor,omitempty" dynamodbav:"eyeColor,omitempty"`
	HairColor string `json:"hairColor,omitempty" dynamodbav:"hairColor,omitempty"`
}

type Stats struct {
	TotalProductions int     `json:"totalProductions" dynamodbav:"totalProductions"`
	TotalViews       int64   `json:"totalViews" dynamodbav:"totalViews"`
	SubscriberCount  int     `json:"subscriberCount" dynamodbav:"subscriberCount"`
	FollowersCount   int     `json:"followersCount" dynamodbav:"followersCount"`
	AverageRating    float64 `json:"averageRating" dynamodbav:"averageRating" validate:"gte=0,lte=10"`
}

type Person struct {
	ID           string       `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	ContentType  ContentType  `json:"contentType" dynamodbav:"contentType" validate:"omitempty,oneof=PERSON"`
	Name         string       `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
	Roles        []EntityType `json:"roles" dynamodbav:"roles" validate:"required,min=1,dive,oneof=PERFORMER CONTENT_CREATOR"`
	StageName    string       `json:"stageName,omitempty" dynamodbav:"stageName,omitempty" validate:"omitempty,max=128"`
	Bio          string       `json:"bio,omitempty" dynamodbav:"bio,omitempty" validate:"omitempty,max=2000"`
	BirthDate    string       `json:"birthDate,omitempty" dynamodbav:"birthDate,omitempty" validate:"omitempty,age18plus"`
	BirthPlace   string       `json:"birthPlace,omitempty" dynamodbav:"birthPlace,omitempty" validate:"omitempty,max=256"`
	Nationality  string       `json:"nationality,omitempty" dynamodbav:"nationality,omitempty" validate:"omitempty,max=64"`
	Gender       Gender       `json:"gender" dynamodbav:"gender" validate:"required,oneof=Male Female Trans"`
	Height       int          `json:"height,omitempty" dynamodbav:"height,omitempty" validate:"omitempty,gte=0"`
	Verified     bool         `json:"verified" dynamodbav:"verified"`
	Active       bool         `json:"active" dynamodbav:"active"`
	DebutYear    int          `json:"debutYear,omitempty" dynamodbav:"debutYear,omitempty" validate:"omitempty,gte=1900,lte=2100"`
	CareerStatus CareerStatus `json:"careerStatus" dynamodbav:"careerStatus" validate:"required,oneof=Active Retired Hiatus"`
	ProfilePath  string       `json:"profilePath,omitempty" dynamodbav:"profilePath,omitempty" validate:"omitempty,max=512"`
	BackdropPath string       `json:"backdropPath,omitempty" dynamodbav:"backdropPath,omitempty" validate:"omitempty,max=512"`
	Measurements Measurements `json:"measurements,omitempty" dynamodbav:"measurements,omitempty"`
	Tags         []EntityRef  `json:"tags" dynamodbav:"tags"`
	Categories   []EntityRef  `json:"categories" dynamodbav:"categories"`
	Specialties  []EntityRef  `json:"specialties" dynamodbav:"specialties"`
	Stats        Stats        `json:"stats" dynamodbav:"stats"`
	Audit        Audit        `json:"audit" dynamodbav:"audit"`
}

type PersonPublicDetail struct {
	ID           string       `json:"id"`
	ContentType  ContentType  `json:"contentType"`
	Name         string       `json:"name"`
	Roles        []EntityType `json:"roles"`
	StageName    string       `json:"stageName,omitempty"`
	Bio          string       `json:"bio,omitempty"`
	BirthDate    string       `json:"birthDate,omitempty"`
	BirthPlace   string       `json:"birthPlace,omitempty"`
	Nationality  string       `json:"nationality,omitempty"`
	Gender       Gender       `json:"gender"`
	Height       int          `json:"height,omitempty"`
	Verified     bool         `json:"verified"`
	Active       bool         `json:"active"`
	DebutYear    int          `json:"debutYear,omitempty"`
	CareerStatus CareerStatus `json:"careerStatus"`
	ProfilePath  string       `json:"profilePath,omitempty"`
	BackdropPath string       `json:"backdropPath,omitempty"`
	Measurements Measurements `json:"measurements,omitempty"`
	Tags         []EntityRef  `json:"tags"`
	Categories   []EntityRef  `json:"categories"`
	Specialties  []EntityRef  `json:"specialties"`
}
