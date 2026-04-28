package model

type PlaybackInfo struct {
	ContentID   string            `json:"contentId"`
	ContentType ContentType       `json:"contentType"`
	Info        PlaybackMeta      `json:"info"`
	Playback    *PlaybackResponse `json:"playback"`
}

type PlaybackMeta struct {
	Title    string      `json:"title"`
	Overview string      `json:"overview"`
	Casts    []EntityRef `json:"casts"`
}

type PlaybackResponse struct {
	DurationSec float64        `json:"durationSec"`
	Streaming   StreamingInfo  `json:"streaming"`
	Video       VideoInfo      `json:"video"`
	Audio       AudioInfo      `json:"audio"`
	Subtitles   SubtitlesInfo  `json:"subtitles"`
	Preview     PreviewInfo    `json:"preview"`
	Thumbnails  ThumbnailsInfo `json:"thumbnails"`
	Sprite      SpriteInfo     `json:"sprite"`
}

type StreamingInfo struct {
	Type           string `json:"type"`
	MasterPlaylist string `json:"masterPlaylist"`
}

type VideoInfo struct {
	Qualities      []string `json:"qualities"`
	DefaultQuality string   `json:"defaultQuality"`
}

type AudioInfo struct {
	Tracks         []AudioTrackInfo `json:"tracks"`
	DefaultTrackID string           `json:"defaultTrackId"`
}

type AudioTrackInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Bitrate  string `json:"bitrate"`
	Default  bool   `json:"default"`
}

type SubtitlesInfo struct {
	Tracks         []SubtitleTrackInfo `json:"tracks"`
	DefaultTrackID *string             `json:"defaultTrackId"`
}

type SubtitleTrackInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Format   string `json:"format"`
	URL      string `json:"url"`
	Default  bool   `json:"default"`
}

type PreviewInfo struct {
	Video string `json:"video"`
}

type ThumbnailsInfo struct {
	Poster string   `json:"poster"`
	Items  []string `json:"items"`
}

type SpriteInfo struct {
	Image string `json:"image"`
	VTT   string `json:"vtt"`
}
