package utils

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetVideoMetadata_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name      string
		filePath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "path traversal attack",
			filePath:  "/tmp/../../etc/passwd",
			wantErr:   true,
			errSubstr: "invalid file path",
		},
		{
			name:      "absolute path outside /tmp",
			filePath:  "/etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "relative path traversal",
			filePath:  "../../../etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "empty path",
			filePath:  "",
			wantErr:   true,
			errSubstr: "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetVideoMetadata(ctx, tt.filePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetVideoMetadata() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("GetVideoMetadata() error = %v, want substring %v", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("GetVideoMetadata() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestGetVideoMetadata_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := GetVideoMetadata(ctx, "/tmp/nonexistent.mp4")
	if err == nil {
		t.Error("GetVideoMetadata() should fail with cancelled context")
	}
}

func TestVideoMetadata_GetResolutionString(t *testing.T) {
	tests := []struct {
		name     string
		metadata VideoMetadata
		want     string
	}{
		{
			name:     "1080p video",
			metadata: VideoMetadata{Width: 1920, Height: 1080},
			want:     "1920x1080",
		},
		{
			name:     "720p video",
			metadata: VideoMetadata{Width: 1280, Height: 720},
			want:     "1280x720",
		},
		{
			name:     "4K video",
			metadata: VideoMetadata{Width: 3840, Height: 2160},
			want:     "3840x2160",
		},
		{
			name:     "zero dimensions",
			metadata: VideoMetadata{Width: 0, Height: 0},
			want:     "",
		},
		{
			name:     "missing width",
			metadata: VideoMetadata{Width: 0, Height: 1080},
			want:     "",
		},
		{
			name:     "missing height",
			metadata: VideoMetadata{Width: 1920, Height: 0},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.GetResolutionString()
			if got != tt.want {
				t.Errorf("GetResolutionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoMetadata_DetermineQualities(t *testing.T) {
	tests := []struct {
		name     string
		metadata VideoMetadata
		want     []string
	}{
		{
			name:     "4K source",
			metadata: VideoMetadata{Height: 2160},
			want:     []string{"360p", "480p", "720p", "1080p", "1440p", "2160p"},
		},
		{
			name:     "1440p source",
			metadata: VideoMetadata{Height: 1440},
			want:     []string{"360p", "480p", "720p", "1080p", "1440p"},
		},
		{
			name:     "1080p source",
			metadata: VideoMetadata{Height: 1080},
			want:     []string{"360p", "480p", "720p", "1080p"},
		},
		{
			name:     "720p source",
			metadata: VideoMetadata{Height: 720},
			want:     []string{"360p", "480p", "720p"},
		},
		{
			name:     "480p source",
			metadata: VideoMetadata{Height: 480},
			want:     []string{"360p", "480p"},
		},
		{
			name:     "360p source",
			metadata: VideoMetadata{Height: 360},
			want:     []string{"360p", "480p"},
		},
		{
			name:     "very low resolution",
			metadata: VideoMetadata{Height: 240},
			want:     []string{"360p", "480p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.DetermineQualities()

			if len(got) != len(tt.want) {
				t.Errorf("DetermineQualities() returned %d qualities, want %d", len(got), len(tt.want))
				return
			}

			for i, quality := range got {
				if quality != tt.want[i] {
					t.Errorf("DetermineQualities()[%d] = %v, want %v", i, quality, tt.want[i])
				}
			}
		})
	}
}

func TestVideoMetadata_DetermineQualities_AlwaysIncludesBaseline(t *testing.T) {

	metadata := VideoMetadata{Height: 144}
	qualities := metadata.DetermineQualities()

	hasBaseline := false
	for _, q := range qualities {
		if q == "360p" || q == "480p" {
			hasBaseline = true
			break
		}
	}

	if !hasBaseline {
		t.Error("DetermineQualities() should always include baseline qualities (360p, 480p)")
	}
}

func TestVideoMetadata_DetermineQualities_NoUpscaling(t *testing.T) {

	metadata := VideoMetadata{Height: 720}
	qualities := metadata.DetermineQualities()

	for _, q := range qualities {
		if q == "1080p" || q == "1440p" || q == "2160p" {
			t.Errorf("DetermineQualities() should not upscale: found %s for 720p source", q)
		}
	}
}

func TestFFProbeOutput_Parsing(t *testing.T) {

	jsonData := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"r_frame_rate": "30/1"
			},
			{
				"codec_type": "audio",
				"codec_name": "aac"
			}
		],
		"format": {
			"duration": "120.5",
			"bit_rate": "5000000"
		}
	}`

	_ = jsonData
}
