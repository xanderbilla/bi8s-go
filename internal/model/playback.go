package model

// PlaybackResponse represents the complete playback information for a content item
type PlaybackResponse struct {
	DurationSec float64            `json:"durationSec"`
	Streaming   StreamingInfo      `json:"streaming"`
	Video       VideoInfo          `json:"video"`
	Audio       AudioInfo          `json:"audio"`
	Subtitles   SubtitlesInfo      `json:"subtitles"`
	Preview     PreviewInfo        `json:"preview"`
	Thumbnails  ThumbnailsInfo     `json:"thumbnails"`
	Sprite      SpriteInfo         `json:"sprite"`
}

// StreamingInfo represents streaming configuration
type StreamingInfo struct {
	Type           string `json:"type"`
	MasterPlaylist string `json:"masterPlaylist"`
}

// VideoInfo represents video quality information
type VideoInfo struct {
	Qualities      []string `json:"qualities"`
	DefaultQuality string   `json:"defaultQuality"`
}

// AudioInfo represents audio track information
type AudioInfo struct {
	Tracks         []AudioTrackInfo `json:"tracks"`
	DefaultTrackID string           `json:"defaultTrackId"`
}

// AudioTrackInfo represents a single audio track
type AudioTrackInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Bitrate  string `json:"bitrate"`
	Default  bool   `json:"default"`
}

// SubtitlesInfo represents subtitle track information
type SubtitlesInfo struct {
	Tracks         []SubtitleTrackInfo `json:"tracks"`
	DefaultTrackID *string             `json:"defaultTrackId"`
}

// SubtitleTrackInfo represents a single subtitle track
type SubtitleTrackInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Format   string `json:"format"`
	URL      string `json:"url"`
	Default  bool   `json:"default"`
}

// PreviewInfo represents preview video information
type PreviewInfo struct {
	Video string `json:"video"`
}

// ThumbnailsInfo represents thumbnail information
type ThumbnailsInfo struct {
	Poster string   `json:"poster"`
	Items  []string `json:"items"`
}

// SpriteInfo represents sprite sheet information
type SpriteInfo struct {
	Image string `json:"image"`
	VTT   string `json:"vtt"`
}
