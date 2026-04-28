package utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "empty path",
			input:     "",
			wantErr:   true,
			errSubstr: "cannot be empty",
		},
		{
			name:      "path traversal with ../",
			input:     "/tmp/../etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "path traversal attempt",
			input:     "/tmp/../../etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "absolute path outside /tmp",
			input:     "/etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "relative path outside /tmp",
			input:     "../../../etc/passwd",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "valid /tmp path",
			input:     "/tmp/video.mp4",
			wantErr:   false,
			errSubstr: "",
		},
		{
			name:      "valid /tmp subdirectory",
			input:     "/tmp/encoding/job123/video.mp4",
			wantErr:   false,
			errSubstr: "",
		},
		{
			name:      "path with .. after cleaning still in /tmp",
			input:     "/tmp/foo/../bar/video.mp4",
			wantErr:   false,
			errSubstr: "",
		},
		{
			name:      "malicious null byte injection",
			input:     "/tmp/video.mp4\x00/etc/passwd",
			wantErr:   true,
			errSubstr: "null byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizePath(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizePath() expected error but got none, result: %s", result)
					return
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("sanitizePath() error = %v, want substring %v", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("sanitizePath() unexpected error = %v", err)
					return
				}
				if !strings.HasPrefix(result, "/tmp/") {
					t.Errorf("sanitizePath() result = %v, want prefix /tmp/", result)
				}
			}
		})
	}
}

func TestTranscodeToHLS_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name      string
		inputPath string
		outputDir string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "input path traversal attack",
			inputPath: "/tmp/../../etc/passwd",
			outputDir: "/tmp/output",
			wantErr:   true,
			errSubstr: "invalid input path",
		},
		{
			name:      "output path traversal attack",
			inputPath: "/tmp/input.mp4",
			outputDir: "/tmp/../../etc",
			wantErr:   true,
			errSubstr: "invalid output path",
		},
		{
			name:      "input outside /tmp",
			inputPath: "/etc/passwd",
			outputDir: "/tmp/output",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
		{
			name:      "output outside /tmp",
			inputPath: "/tmp/input.mp4",
			outputDir: "/var/www/html",
			wantErr:   true,
			errSubstr: "outside allowed directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TranscodeToHLS(ctx, tt.inputPath, tt.outputDir, "720p", "1280x720", "2500k")

			if tt.wantErr {
				if err == nil {
					t.Errorf("TranscodeToHLS() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("TranscodeToHLS() error = %v, want substring %v", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("TranscodeToHLS() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestTranscodeAudioToHLS_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name      string
		inputPath string
		outputDir string
		wantErr   bool
	}{
		{
			name:      "path traversal in input",
			inputPath: "../../../etc/passwd",
			outputDir: "/tmp/output",
			wantErr:   true,
		},
		{
			name:      "path traversal in output",
			inputPath: "/tmp/audio.mp3",
			outputDir: "/tmp/../../../etc",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TranscodeAudioToHLS(ctx, tt.inputPath, tt.outputDir, "128k")

			if tt.wantErr && err == nil {
				t.Errorf("TranscodeAudioToHLS() expected error but got none")
			}
		})
	}
}

func TestGenerateThumbnail_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name       string
		inputPath  string
		outputPath string
		wantErr    bool
	}{
		{
			name:       "input path traversal",
			inputPath:  "/tmp/../../etc/passwd",
			outputPath: "/tmp/thumb.jpg",
			wantErr:    true,
		},
		{
			name:       "output path traversal",
			inputPath:  "/tmp/video.mp4",
			outputPath: "/tmp/../../var/www/thumb.jpg",
			wantErr:    true,
		},
		{
			name:       "both paths malicious",
			inputPath:  "../../../etc/passwd",
			outputPath: "../../../var/www/thumb.jpg",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateThumbnail(ctx, tt.inputPath, tt.outputPath, 10.0)

			if tt.wantErr && err == nil {
				t.Errorf("GenerateThumbnail() expected error but got none")
			}
		})
	}
}

func TestGeneratePreview_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := GeneratePreview(ctx, "/etc/passwd", "/tmp/preview.mp4", 30.0)
	if err == nil {
		t.Error("GeneratePreview() should reject path outside /tmp")
	}

	err = GeneratePreview(ctx, "/tmp/video.mp4", "/var/www/preview.mp4", 30.0)
	if err == nil {
		t.Error("GeneratePreview() should reject output path outside /tmp")
	}
}

func TestGenerateSprite_PathValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		input   string
		image   string
		vtt     string
		wantErr bool
	}{
		{
			name:    "input outside /tmp",
			input:   "/etc/passwd",
			image:   "/tmp/sprite.jpg",
			vtt:     "/tmp/sprite.vtt",
			wantErr: true,
		},
		{
			name:    "image output outside /tmp",
			input:   "/tmp/video.mp4",
			image:   "/var/www/sprite.jpg",
			vtt:     "/tmp/sprite.vtt",
			wantErr: true,
		},
		{
			name:    "vtt output outside /tmp",
			input:   "/tmp/video.mp4",
			image:   "/tmp/sprite.jpg",
			vtt:     "/var/www/sprite.vtt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateSprite(ctx, tt.input, tt.image, tt.vtt, 60.0)

			if tt.wantErr && err == nil {
				t.Errorf("GenerateSprite() expected error but got none")
			}
		})
	}
}

func TestGenerateMasterPlaylist_PathValidation(t *testing.T) {
	qualities := []QualityPlaylist{
		{Quality: "720p", Resolution: "1280x720", Bandwidth: 2500000, RelativePath: "720p/index.m3u8"},
	}

	tests := []struct {
		name       string
		outputPath string
		wantErr    bool
	}{
		{
			name:       "output outside /tmp",
			outputPath: "/var/www/master.m3u8",
			wantErr:    true,
		},
		{
			name:       "path traversal",
			outputPath: "/tmp/../../etc/master.m3u8",
			wantErr:    true,
		},
		{
			name:       "valid /tmp path",
			outputPath: "/tmp/test_master.m3u8",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateMasterPlaylist(tt.outputPath, qualities, []AudioPlaylist{})

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateMasterPlaylist() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("GenerateMasterPlaylist() unexpected error = %v", err)
				} else {

					os.Remove(tt.outputPath)
				}
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tmpFile := filepath.Join("/tmp", "test_cancel.mp4")
	if err := os.WriteFile(tmpFile, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	outputDir := filepath.Join("/tmp", "test_cancel_output")
	defer os.RemoveAll(outputDir)

	err := TranscodeToHLS(ctx, tmpFile, outputDir, "720p", "1280x720", "2500k")
	if err == nil {
		t.Error("TranscodeToHLS() should fail with cancelled context")
	}
}

func TestCountSegments(t *testing.T) {

	tmpDir := filepath.Join("/tmp", "test_segments")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, "segment_"+string(rune('0'+i))+".ts")
		if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test segment: %v", err)
		}
	}

	count := CountSegments(tmpDir)
	if count != 5 {
		t.Errorf("CountSegments() = %d, want 5", count)
	}

	count = CountSegments("/tmp/nonexistent")
	if count != 0 {
		t.Errorf("CountSegments() for non-existent dir = %d, want 0", count)
	}
}

func TestBoolToYesNo(t *testing.T) {
	if boolToYesNo(true) != "YES" {
		t.Error("boolToYesNo(true) should return YES")
	}
	if boolToYesNo(false) != "NO" {
		t.Error("boolToYesNo(false) should return NO")
	}
}
