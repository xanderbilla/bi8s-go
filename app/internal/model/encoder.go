package model

import "time"

type EncoderJobStatus string

const (
	EncoderStatusQueued                EncoderJobStatus = "QUEUED"
	EncoderStatusProcessing            EncoderJobStatus = "PROCESSING"
	EncoderStatusCompleted             EncoderJobStatus = "COMPLETED"
	EncoderStatusCompletedWithWarnings EncoderJobStatus = "COMPLETED_WITH_WARNINGS"
	EncoderStatusFailed                EncoderJobStatus = "FAILED"
	EncoderStatusCancelled             EncoderJobStatus = "CANCELLED"
)

type EncoderJob struct {
	JobID       string            `json:"jobId" dynamodbav:"id"`
	ContentType ContentType       `json:"contentType" dynamodbav:"contentType"`
	ContentID   string            `json:"contentId" dynamodbav:"contentId"`
	Status      EncoderJobStatus  `json:"status" dynamodbav:"status"`
	Input       EncoderInput      `json:"input" dynamodbav:"input"`
	Output      EncoderOutput     `json:"output" dynamodbav:"output"`
	Playback    *PlaybackResponse `json:"playback,omitempty" dynamodbav:"playback,omitempty"`
	Errors      []EncoderError    `json:"errors" dynamodbav:"errors"`
	Warnings    []EncoderWarning  `json:"warnings" dynamodbav:"warnings"`
	Meta        EncoderMeta       `json:"meta" dynamodbav:"meta"`

	Version int `json:"version" dynamodbav:"version"`
}

type EncoderInput struct {
	FileName           string          `json:"fileName" dynamodbav:"fileName"`
	SourcePath         string          `json:"sourcePath" dynamodbav:"sourcePath"`
	SourceExtension    string          `json:"sourceExtension" dynamodbav:"sourceExtension"`
	Resolution         string          `json:"resolution" dynamodbav:"resolution"`
	DurationSec        float64         `json:"durationSec" dynamodbav:"durationSec"`
	VideoCodec         string          `json:"videoCodec" dynamodbav:"videoCodec"`
	AudioCodec         string          `json:"audioCodec" dynamodbav:"audioCodec"`
	HasEmbeddedAudio   bool            `json:"hasEmbeddedAudio" dynamodbav:"hasEmbeddedAudio"`
	HasExternalAudio   bool            `json:"hasExternalAudio" dynamodbav:"hasExternalAudio"`
	HasSubtitles       bool            `json:"hasSubtitles" dynamodbav:"hasSubtitles"`
	GeneratedQualities []QualityConfig `json:"generatedQualities" dynamodbav:"generatedQualities"`
}

type QualityConfig struct {
	Quality      string `json:"quality" dynamodbav:"quality"`
	Resolution   string `json:"resolution" dynamodbav:"resolution"`
	VideoBitrate string `json:"videoBitrate" dynamodbav:"videoBitrate"`
}

type EncoderOutput struct {
	BaseOutputDir   string           `json:"baseOutputDir" dynamodbav:"baseOutputDir"`
	MasterPlaylists MasterPlaylists  `json:"masterPlaylists" dynamodbav:"masterPlaylists"`
	Video           VideoOutput      `json:"video" dynamodbav:"video"`
	Audio           AudioOutput      `json:"audio" dynamodbav:"audio"`
	Subtitles       SubtitlesOutput  `json:"subtitles" dynamodbav:"subtitles"`
	Thumbnails      ThumbnailsOutput `json:"thumbnails" dynamodbav:"thumbnails"`
	Preview         PreviewOutput    `json:"preview" dynamodbav:"preview"`
	Sprite          SpriteOutput     `json:"sprite" dynamodbav:"sprite"`
}

type MasterPlaylists struct {
	RecordMasterPlaylist  *string `json:"recordMasterPlaylist" dynamodbav:"recordMasterPlaylist"`
	CurrentMasterPlaylist *string `json:"currentMasterPlaylist" dynamodbav:"currentMasterPlaylist"`
}

type VideoOutput struct {
	Qualities []VideoQuality `json:"qualities" dynamodbav:"qualities"`
}

type VideoQuality struct {
	Quality    string `json:"quality" dynamodbav:"quality"`
	Resolution string `json:"resolution" dynamodbav:"resolution"`
	Playlist   string `json:"playlist" dynamodbav:"playlist"`
	ChunksDir  string `json:"chunksDir" dynamodbav:"chunksDir"`
	ChunkCount int    `json:"chunkCount" dynamodbav:"chunkCount"`
}

type AudioOutput struct {
	Tracks []AudioTrack `json:"tracks" dynamodbav:"tracks"`
}

type AudioTrack struct {
	Bitrate    string `json:"bitrate" dynamodbav:"bitrate"`
	Playlist   string `json:"playlist" dynamodbav:"playlist"`
	ChunksDir  string `json:"chunksDir" dynamodbav:"chunksDir"`
	ChunkCount int    `json:"chunkCount" dynamodbav:"chunkCount"`
	Language   string `json:"language" dynamodbav:"language"`
	Label      string `json:"label" dynamodbav:"label"`
	Default    bool   `json:"default" dynamodbav:"default"`
}

type SubtitlesOutput struct {
	Dir    string           `json:"dir" dynamodbav:"dir"`
	Tracks []SubtitleOutput `json:"tracks" dynamodbav:"tracks"`
}

type SubtitleOutput struct {
	Language string `json:"language" dynamodbav:"language"`
	Label    string `json:"label" dynamodbav:"label"`
	Format   string `json:"format" dynamodbav:"format"`
	Path     string `json:"path" dynamodbav:"path"`
	Default  bool   `json:"default" dynamodbav:"default"`
}

type ThumbnailsOutput struct {
	Dir   string   `json:"dir" dynamodbav:"dir"`
	Count int      `json:"count" dynamodbav:"count"`
	Items []string `json:"items" dynamodbav:"items"`
}

type PreviewOutput struct {
	File        *string  `json:"file" dynamodbav:"file"`
	DurationSec *float64 `json:"durationSec" dynamodbav:"durationSec"`
}

type SpriteOutput struct {
	Image *string `json:"image" dynamodbav:"image"`
	VTT   *string `json:"vtt" dynamodbav:"vtt"`
}

type EncoderError struct {
	Code    string  `json:"code" dynamodbav:"code"`
	Message string  `json:"message" dynamodbav:"message"`
	Stage   string  `json:"stage" dynamodbav:"stage"`
	Quality *string `json:"quality" dynamodbav:"quality"`
	Path    string  `json:"path" dynamodbav:"path"`
	Details string  `json:"details" dynamodbav:"details"`
}

type EncoderWarning struct {
	Code    string  `json:"code" dynamodbav:"code"`
	Message string  `json:"message" dynamodbav:"message"`
	Stage   string  `json:"stage" dynamodbav:"stage"`
	Quality *string `json:"quality" dynamodbav:"quality"`
	Path    string  `json:"path" dynamodbav:"path"`
	Details string  `json:"details" dynamodbav:"details"`
}

type EncoderMeta struct {
	CreatedAt         time.Time  `json:"createdAt" dynamodbav:"createdAt"`
	StartedAt         *time.Time `json:"startedAt" dynamodbav:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt" dynamodbav:"completedAt"`
	FailedAt          *time.Time `json:"failedAt" dynamodbav:"failedAt"`
	ProcessingTimeSec *float64   `json:"processingTimeSec" dynamodbav:"processingTimeSec"`
}
