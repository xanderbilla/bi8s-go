package model

import "time"

// EncoderJobStatus represents the status of an encoding job
type EncoderJobStatus string

const (
	EncoderStatusQueued              EncoderJobStatus = "QUEUED"
	EncoderStatusProcessing          EncoderJobStatus = "PROCESSING"
	EncoderStatusCompleted           EncoderJobStatus = "COMPLETED"
	EncoderStatusCompletedWithWarnings EncoderJobStatus = "COMPLETED_WITH_WARNINGS"
	EncoderStatusFailed              EncoderJobStatus = "FAILED"
	EncoderStatusCancelled           EncoderJobStatus = "CANCELLED"
)

// EncoderJob represents a video encoding job
type EncoderJob struct {
	JobID       string            `json:"jobId" dynamodbav:"jobId"`
	ContentType ContentType       `json:"contentType" dynamodbav:"contentType"`
	ContentID   string            `json:"contentId" dynamodbav:"contentId"`
	Status      EncoderJobStatus  `json:"status" dynamodbav:"status"`
	Input       EncoderInput      `json:"input" dynamodbav:"input"`
	Output      EncoderOutput     `json:"output" dynamodbav:"output"`
	Playback    *PlaybackResponse `json:"playback,omitempty" dynamodbav:"playback,omitempty"`
	Errors      []EncoderError    `json:"errors" dynamodbav:"errors"`
	Warnings    []EncoderWarning  `json:"warnings" dynamodbav:"warnings"`
	Meta        EncoderMeta       `json:"meta" dynamodbav:"meta"`
}

// EncoderInput represents input video information
type EncoderInput struct {
	FileName            string            `json:"fileName" dynamodbav:"fileName"`
	SourcePath          string            `json:"sourcePath" dynamodbav:"sourcePath"`
	SourceExtension     string            `json:"sourceExtension" dynamodbav:"sourceExtension"`
	Resolution          string            `json:"resolution" dynamodbav:"resolution"`
	DurationSec         float64           `json:"durationSec" dynamodbav:"durationSec"`
	VideoCodec          string            `json:"videoCodec" dynamodbav:"videoCodec"`
	AudioCodec          string            `json:"audioCodec" dynamodbav:"audioCodec"`
	HasEmbeddedAudio    bool              `json:"hasEmbeddedAudio" dynamodbav:"hasEmbeddedAudio"`
	HasExternalAudio    bool              `json:"hasExternalAudio" dynamodbav:"hasExternalAudio"`
	HasSubtitles        bool              `json:"hasSubtitles" dynamodbav:"hasSubtitles"`
	GeneratedQualities  []QualityConfig   `json:"generatedQualities" dynamodbav:"generatedQualities"`
}

// QualityConfig represents a video quality configuration
type QualityConfig struct {
	Quality      string `json:"quality" dynamodbav:"quality"`
	Resolution   string `json:"resolution" dynamodbav:"resolution"`
	VideoBitrate string `json:"videoBitrate" dynamodbav:"videoBitrate"`
}

// EncoderOutput represents all output files and paths
type EncoderOutput struct {
	BaseOutputDir   string              `json:"baseOutputDir" dynamodbav:"baseOutputDir"`
	MasterPlaylists MasterPlaylists     `json:"masterPlaylists" dynamodbav:"masterPlaylists"`
	Video           VideoOutput         `json:"video" dynamodbav:"video"`
	Audio           AudioOutput         `json:"audio" dynamodbav:"audio"`
	Subtitles       SubtitlesOutput     `json:"subtitles" dynamodbav:"subtitles"`
	Thumbnails      ThumbnailsOutput    `json:"thumbnails" dynamodbav:"thumbnails"`
	Preview         PreviewOutput       `json:"preview" dynamodbav:"preview"`
	Sprite          SpriteOutput        `json:"sprite" dynamodbav:"sprite"`
}

// MasterPlaylists represents master playlist paths
type MasterPlaylists struct {
	RecordMasterPlaylist  *string `json:"recordMasterPlaylist" dynamodbav:"recordMasterPlaylist"`
	CurrentMasterPlaylist *string `json:"currentMasterPlaylist" dynamodbav:"currentMasterPlaylist"`
}

// VideoOutput represents video renditions
type VideoOutput struct {
	Qualities []VideoQuality `json:"qualities" dynamodbav:"qualities"`
}

// VideoQuality represents a single video quality rendition
type VideoQuality struct {
	Quality    string `json:"quality" dynamodbav:"quality"`
	Resolution string `json:"resolution" dynamodbav:"resolution"`
	Playlist   string `json:"playlist" dynamodbav:"playlist"`
	ChunksDir  string `json:"chunksDir" dynamodbav:"chunksDir"`
	ChunkCount int    `json:"chunkCount" dynamodbav:"chunkCount"`
}

// AudioOutput represents audio tracks
type AudioOutput struct {
	Tracks []AudioTrack `json:"tracks" dynamodbav:"tracks"`
}

// AudioTrack represents a single audio track
type AudioTrack struct {
	Bitrate    string `json:"bitrate" dynamodbav:"bitrate"`
	Playlist   string `json:"playlist" dynamodbav:"playlist"`
	ChunksDir  string `json:"chunksDir" dynamodbav:"chunksDir"`
	ChunkCount int    `json:"chunkCount" dynamodbav:"chunkCount"`
	Language   string `json:"language" dynamodbav:"language"`
	Label      string `json:"label" dynamodbav:"label"`
	Default    bool   `json:"default" dynamodbav:"default"`
}

// SubtitlesOutput represents subtitle tracks
type SubtitlesOutput struct {
	Dir    string `json:"dir" dynamodbav:"dir"`
	Tracks []any  `json:"tracks" dynamodbav:"tracks"`
}

// ThumbnailsOutput represents generated thumbnails
type ThumbnailsOutput struct {
	Dir   string   `json:"dir" dynamodbav:"dir"`
	Count int      `json:"count" dynamodbav:"count"`
	Items []string `json:"items" dynamodbav:"items"`
}

// PreviewOutput represents preview video
type PreviewOutput struct {
	File        *string  `json:"file" dynamodbav:"file"`
	DurationSec *float64 `json:"durationSec" dynamodbav:"durationSec"`
}

// SpriteOutput represents sprite image and VTT
type SpriteOutput struct {
	Image *string `json:"image" dynamodbav:"image"`
	VTT   *string `json:"vtt" dynamodbav:"vtt"`
}

// EncoderError represents an error during encoding
type EncoderError struct {
	Code    string  `json:"code" dynamodbav:"code"`
	Message string  `json:"message" dynamodbav:"message"`
	Stage   string  `json:"stage" dynamodbav:"stage"`
	Quality *string `json:"quality" dynamodbav:"quality"`
	Path    string  `json:"path" dynamodbav:"path"`
	Details string  `json:"details" dynamodbav:"details"`
}

// EncoderWarning represents a warning during encoding
type EncoderWarning struct {
	Code    string  `json:"code" dynamodbav:"code"`
	Message string  `json:"message" dynamodbav:"message"`
	Stage   string  `json:"stage" dynamodbav:"stage"`
	Quality *string `json:"quality" dynamodbav:"quality"`
	Path    string  `json:"path" dynamodbav:"path"`
	Details string  `json:"details" dynamodbav:"details"`
}

// EncoderMeta represents metadata about the encoding job
type EncoderMeta struct {
	CreatedAt        time.Time  `json:"createdAt" dynamodbav:"createdAt"`
	StartedAt        *time.Time `json:"startedAt" dynamodbav:"startedAt"`
	CompletedAt      *time.Time `json:"completedAt" dynamodbav:"completedAt"`
	FailedAt         *time.Time `json:"failedAt" dynamodbav:"failedAt"`
	ProcessingTimeSec *float64   `json:"processingTimeSec" dynamodbav:"processingTimeSec"`
}
