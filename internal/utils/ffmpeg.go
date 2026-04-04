package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// TranscodeToHLS transcodes a video to HLS format with specified quality
func TranscodeToHLS(inputPath, outputDir, quality, resolution, videoBitrate string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	playlistPath := filepath.Join(outputDir, "index.m3u8")
	
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-c:v", "libx264",
		"-b:v", videoBitrate,
		"-s", resolution,
		"-c:a", "aac",
		"-b:a", "128k",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
		playlistPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg transcode failed for %s: %w, output: %s", quality, err, string(output))
	}

	return nil
}

// TranscodeAudioToHLS transcodes audio to HLS format with specified bitrate
func TranscodeAudioToHLS(inputPath, outputDir, bitrate string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	playlistPath := filepath.Join(outputDir, "index.m3u8")
	
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", bitrate,
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
		playlistPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg audio transcode failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GenerateThumbnail generates a thumbnail at a specific timestamp
func GenerateThumbnail(inputPath, outputPath string, timestamp float64) error {
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-vframes", "1",
		"-q:v", "2",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GeneratePreview generates a preview video of specified duration
func GeneratePreview(inputPath, outputPath string, duration float64) error {
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-t", fmt.Sprintf("%.2f", duration),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:v", "1000k",
		"-b:a", "128k",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg preview generation failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GenerateSprite generates a sprite sheet and VTT file
func GenerateSprite(inputPath, spriteImagePath, spriteVTTPath string, duration float64) error {
	outputDir := filepath.Dir(spriteImagePath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create sprite directory: %w", err)
	}

	// Generate 10 frames evenly distributed
	frameCount := 10
	interval := duration / float64(frameCount)
	
	// Extract frames
	framesDir := filepath.Join(outputDir, "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return fmt.Errorf("failed to create frames directory: %w", err)
	}
	defer os.RemoveAll(framesDir)

	for i := 0; i < frameCount; i++ {
		timestamp := float64(i) * interval
		framePath := filepath.Join(framesDir, fmt.Sprintf("frame_%03d.jpg", i))
		
		cmd := exec.Command("ffmpeg",
			"-i", inputPath,
			"-ss", fmt.Sprintf("%.2f", timestamp),
			"-vframes", "1",
			"-s", "160x90",
			framePath,
		)
		
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to extract frame %d: %w, output: %s", i, err, string(output))
		}
	}

	// Create sprite sheet using montage (ImageMagick) or ffmpeg tile
	cmd := exec.Command("ffmpeg",
		"-i", filepath.Join(framesDir, "frame_%03d.jpg"),
		"-filter_complex", fmt.Sprintf("tile=%dx%d", 5, 2),
		spriteImagePath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create sprite sheet: %w, output: %s", err, string(output))
	}

	// Generate VTT file
	vttContent := "WEBVTT\n\n"
	for i := 0; i < frameCount; i++ {
		startTime := float64(i) * interval
		endTime := startTime + interval
		x := (i % 5) * 160
		y := (i / 5) * 90
		
		vttContent += fmt.Sprintf("%02d:%02d:%02d.000 --> %02d:%02d:%02d.000\n",
			int(startTime)/3600, (int(startTime)%3600)/60, int(startTime)%60,
			int(endTime)/3600, (int(endTime)%3600)/60, int(endTime)%60)
		vttContent += fmt.Sprintf("%s#xywh=%d,%d,160,90\n\n", filepath.Base(spriteImagePath), x, y)
	}

	if err := os.WriteFile(spriteVTTPath, []byte(vttContent), 0644); err != nil {
		return fmt.Errorf("failed to write VTT file: %w", err)
	}

	return nil
}

// GenerateMasterPlaylist generates an HLS master playlist
func GenerateMasterPlaylist(outputPath string, qualities []QualityPlaylist, audioTracks []AudioPlaylist) error {
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create playlist directory: %w", err)
	}

	content := "#EXTM3U\n#EXT-X-VERSION:3\n\n"

	// Add video variants
	for _, q := range qualities {
		content += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n", q.Bandwidth, q.Resolution)
		content += fmt.Sprintf("%s\n\n", q.RelativePath)
	}

	// Add audio tracks if available
	for _, a := range audioTracks {
		content += fmt.Sprintf("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",DEFAULT=%s,URI=\"%s\"\n",
			a.Label, boolToYesNo(a.Default), a.RelativePath)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write master playlist: %w", err)
	}

	return nil
}

// QualityPlaylist represents a video quality variant
type QualityPlaylist struct {
	Quality      string
	Resolution   string
	Bandwidth    int
	RelativePath string
}

// AudioPlaylist represents an audio track
type AudioPlaylist struct {
	Label        string
	Default      bool
	RelativePath string
}

func boolToYesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

// CountSegments counts the number of .ts segments in a directory
func CountSegments(dir string) int {
	files, err := filepath.Glob(filepath.Join(dir, "segment_*.ts"))
	if err != nil {
		return 0
	}
	return len(files)
}
