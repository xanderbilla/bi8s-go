package utils

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// VideoMetadata contains information about a video file
type VideoMetadata struct {
	Duration     float64
	Width        int
	Height       int
	VideoCodec   string
	AudioCodec   string
	HasAudio     bool
	HasVideo     bool
	Bitrate      int64
	FrameRate    float64
}

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

// GetVideoMetadata extracts metadata from a video file using ffprobe
func GetVideoMetadata(filePath string) (*VideoMetadata, error) {
	// Run ffprobe to get video information in JSON format
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probeData FFProbeOutput
	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	metadata := &VideoMetadata{}

	// Parse duration
	if probeData.Format.Duration != "" {
		duration, err := strconv.ParseFloat(probeData.Format.Duration, 64)
		if err == nil {
			metadata.Duration = duration
		}
	}

	// Parse bitrate
	if probeData.Format.BitRate != "" {
		bitrate, err := strconv.ParseInt(probeData.Format.BitRate, 10, 64)
		if err == nil {
			metadata.Bitrate = bitrate
		}
	}

	// Parse streams
	for _, stream := range probeData.Streams {
		switch stream.CodecType {
		case "video":
			metadata.HasVideo = true
			metadata.VideoCodec = stream.CodecName
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			
			// Parse frame rate
			if stream.RFrameRate != "" {
				parts := strings.Split(stream.RFrameRate, "/")
				if len(parts) == 2 {
					num, err1 := strconv.ParseFloat(parts[0], 64)
					den, err2 := strconv.ParseFloat(parts[1], 64)
					if err1 == nil && err2 == nil && den != 0 {
						metadata.FrameRate = num / den
					}
				}
			}
		case "audio":
			metadata.HasAudio = true
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = stream.CodecName
			}
		}
	}

	return metadata, nil
}

// GetResolutionString returns a formatted resolution string (e.g., "1920x1080")
func (m *VideoMetadata) GetResolutionString() string {
	if m.Width > 0 && m.Height > 0 {
		return fmt.Sprintf("%dx%d", m.Width, m.Height)
	}
	return ""
}

// DetermineQualities returns appropriate quality levels based on video resolution
func (m *VideoMetadata) DetermineQualities() []string {
	qualities := []string{}
	
	// Always include lower qualities
	qualities = append(qualities, "360p", "480p")
	
	// Add 720p if source is 720p or higher
	if m.Height >= 720 {
		qualities = append(qualities, "720p")
	}
	
	// Add 1080p if source is 1080p or higher
	if m.Height >= 1080 {
		qualities = append(qualities, "1080p")
	}
	
	// Add 1440p if source is 1440p or higher
	if m.Height >= 1440 {
		qualities = append(qualities, "1440p")
	}
	
	// Add 4K if source is 4K or higher
	if m.Height >= 2160 {
		qualities = append(qualities, "2160p")
	}
	
	return qualities
}
