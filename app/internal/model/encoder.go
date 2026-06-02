package model

// EncoderJob is a single row from the encoder DynamoDB table.
// Only the fields needed by the API are mapped here.
type EncoderJob struct {
	ID          string       `dynamodbav:"id"`
	ContentID   string       `dynamodbav:"contentId"`
	ContentType string       `dynamodbav:"contentType"`
	Status      string       `dynamodbav:"status"`
	Playback    PlaybackInfo `dynamodbav:"playback"`
}

// PlaybackInfo is returned verbatim to consumers, with all path keys
// replaced by presigned URLs before leaving the service layer.
type PlaybackInfo struct {
	DurationSec float64            `json:"durationSec"   dynamodbav:"durationSec"`
	Streaming   PlaybackStreaming   `json:"streaming"     dynamodbav:"streaming"`
	Video       PlaybackVideo       `json:"video"         dynamodbav:"video"`
	Audio       PlaybackAudio       `json:"audio"         dynamodbav:"audio"`
	Subtitles   PlaybackSubtitles   `json:"subtitles"     dynamodbav:"subtitles"`
	Thumbnails  PlaybackThumbnails  `json:"thumbnails"    dynamodbav:"thumbnails"`
	Preview     PlaybackPreview     `json:"preview"       dynamodbav:"preview"`
	Sprite      PlaybackSprite      `json:"sprite"        dynamodbav:"sprite"`
}

// PlaybackStreaming holds the HLS master playlist URL.
type PlaybackStreaming struct {
	Type           string `json:"type"           dynamodbav:"type"`
	MasterPlaylist string `json:"masterPlaylist" dynamodbav:"masterPlaylist"`
}

// PlaybackVideo describes the available quality levels.
type PlaybackVideo struct {
	DefaultQuality string   `json:"defaultQuality" dynamodbav:"defaultQuality"`
	Qualities      []string `json:"qualities"      dynamodbav:"qualities"`
}

// PlaybackAudioTrack is one audio rendition.
type PlaybackAudioTrack struct {
	Bitrate  string `json:"bitrate"   dynamodbav:"bitrate"`
	Default  bool   `json:"default"   dynamodbav:"default"`
	ID       string `json:"id"        dynamodbav:"id"`
	Label    string `json:"label"     dynamodbav:"label"`
	Language string `json:"language"  dynamodbav:"language"`
}

// PlaybackAudio holds all audio tracks.
type PlaybackAudio struct {
	DefaultTrackID string               `json:"defaultTrackId" dynamodbav:"defaultTrackId"`
	Tracks         []PlaybackAudioTrack `json:"tracks"         dynamodbav:"tracks"`
}

// PlaybackSubtitleTrack is one subtitle rendition.
type PlaybackSubtitleTrack struct {
	ID       string `json:"id"       dynamodbav:"id"`
	Label    string `json:"label"    dynamodbav:"label"`
	Language string `json:"language" dynamodbav:"language"`
	URL      string `json:"url"      dynamodbav:"url"`
}

// PlaybackSubtitles holds all subtitle tracks.
type PlaybackSubtitles struct {
	DefaultTrackID string                  `json:"defaultTrackId" dynamodbav:"defaultTrackId"`
	Tracks         []PlaybackSubtitleTrack `json:"tracks"         dynamodbav:"tracks"`
}

// PlaybackThumbnails holds presigned thumbnail URLs.
type PlaybackThumbnails struct {
	Items []string `json:"items" dynamodbav:"items"`
}

// PlaybackPreview is an optional short preview clip.
type PlaybackPreview struct {
	DurationSec float64 `json:"durationSec" dynamodbav:"durationSec"`
	URL         string  `json:"url"         dynamodbav:"url"`
}

// PlaybackSprite holds the sprite sheet and VTT chapter file.
type PlaybackSprite struct {
	Image string `json:"image" dynamodbav:"image"`
	VTT   string `json:"vtt"   dynamodbav:"vtt"`
}
