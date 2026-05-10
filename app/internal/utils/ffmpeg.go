package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/xanderbilla/bi8s-go/internal/env"
)

const ffmpegStderrCap = 64 << 10

var (
	qualityRe    = regexp.MustCompile(`^[0-9]{3,4}p$`)
	resolutionRe = regexp.MustCompile(`^[0-9]{2,5}x[0-9]{2,5}$`)
	bitrateRe    = regexp.MustCompile(`^[0-9]{2,6}[kKmM]$`)

	tmpDirOnce sync.Once
	tmpDirVal  string
)

func TmpDir() string {
	tmpDirOnce.Do(func() {
		v := strings.TrimSpace(env.GetString("BI8S_TMP_DIR", ""))
		if v == "" {
			v = os.TempDir()
		}

		if abs, err := filepath.Abs(v); err == nil {
			v = filepath.Clean(abs)
		}
		tmpDirVal = v
	})
	return tmpDirVal
}

func runFFmpeg(cmd *exec.Cmd) ([]byte, error) {
	var buf bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &limitedWriter{w: &buf, n: ffmpegStderrCap}
	err := cmd.Run()
	return buf.Bytes(), err
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {

		return len(p), nil
	}
	truncated := false
	original := len(p)
	if len(p) > l.n {
		p = p[:l.n]
		truncated = true
	}
	n, err := l.w.Write(p)
	l.n -= n
	if err != nil {
		return n, err
	}
	if truncated {

		return original, nil
	}
	return n, nil
}

func sanitizePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path cannot be empty")
	}

	if strings.Contains(path, "\x00") {
		return "", errors.New("path contains null byte")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	abs = filepath.Clean(abs)

	root := TmpDir()
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside allowed directory: %s", abs)
	}

	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	if rootErr != nil {
		resolvedRoot = root
	}

	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
			return "", fmt.Errorf("path outside allowed directory after symlink resolution: %s", resolved)
		}
	}

	if strings.Contains(abs, "..") {
		return "", fmt.Errorf("path traversal detected: %s", abs)
	}

	return abs, nil
}

func TranscodeToHLS(ctx context.Context, inputPath, outputDir, quality, resolution, videoBitrate string) error {
	if !qualityRe.MatchString(quality) {
		return fmt.Errorf("invalid quality token: %q", quality)
	}
	if !resolutionRe.MatchString(resolution) {
		return fmt.Errorf("invalid resolution token: %q", resolution)
	}
	if !bitrateRe.MatchString(videoBitrate) {
		return fmt.Errorf("invalid bitrate token: %q", videoBitrate)
	}

	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeOutput, err := sanitizePath(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	if err := os.MkdirAll(safeOutput, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	playlistPath := filepath.Join(safeOutput, "index.m3u8")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-i", safeInput,
		"-c:v", "libx264",
		"-b:v", videoBitrate,
		"-s", resolution,
		"-c:a", "aac",
		"-b:a", "128k",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(safeOutput, "segment_%03d.ts"),
		playlistPath,
	)

	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg transcode failed for %s: %w, stderr: %s", quality, err, string(stderr))
	}

	return nil
}

func TranscodeAudioToHLS(ctx context.Context, inputPath, outputDir, bitrate string) error {
	if !bitrateRe.MatchString(bitrate) {
		return fmt.Errorf("invalid bitrate token: %q", bitrate)
	}

	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeOutput, err := sanitizePath(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	if err := os.MkdirAll(safeOutput, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	playlistPath := filepath.Join(safeOutput, "index.m3u8")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-i", safeInput,
		"-vn",
		"-c:a", "aac",
		"-b:a", bitrate,
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(safeOutput, "segment_%03d.ts"),
		playlistPath,
	)

	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg audio transcode failed: %w, stderr: %s", err, string(stderr))
	}

	return nil
}

func GenerateThumbnail(ctx context.Context, inputPath, outputPath string, timestamp float64) error {
	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeOutput, err := sanitizePath(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	outputDir := filepath.Dir(safeOutput)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", safeInput,
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-vframes", "1",
		"-q:v", "2",
		safeOutput,
	)

	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %w, stderr: %s", err, string(stderr))
	}

	return nil
}

func GeneratePreview(ctx context.Context, inputPath, outputPath string, duration float64) error {
	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeOutput, err := sanitizePath(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	outputDir := filepath.Dir(safeOutput)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", safeInput,
		"-t", fmt.Sprintf("%.2f", duration),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:v", "1000k",
		"-b:a", "128k",
		safeOutput,
	)

	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg preview generation failed: %w, stderr: %s", err, string(stderr))
	}

	return nil
}

func GenerateSprite(ctx context.Context, inputPath, spriteImagePath, spriteVTTPath string, duration float64) error {
	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeImagePath, err := sanitizePath(spriteImagePath)
	if err != nil {
		return fmt.Errorf("invalid sprite image path: %w", err)
	}

	safeVTTPath, err := sanitizePath(spriteVTTPath)
	if err != nil {
		return fmt.Errorf("invalid VTT path: %w", err)
	}

	outputDir := filepath.Dir(safeImagePath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create sprite directory: %w", err)
	}

	frameCount := 10
	interval := duration / float64(frameCount)

	framesDir := filepath.Join(outputDir, "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return fmt.Errorf("failed to create frames directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(framesDir); err != nil && !os.IsNotExist(err) {
			slog.WarnContext(ctx, "failed to remove frames directory", "frames_dir", framesDir, "error", err.Error())
		}
	}()

	for i := 0; i < frameCount; i++ {
		timestamp := float64(i) * interval
		framePath := filepath.Join(framesDir, fmt.Sprintf("frame_%03d.jpg", i))

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", safeInput,
			"-ss", fmt.Sprintf("%.2f", timestamp),
			"-vframes", "1",
			"-s", "160x90",
			framePath,
		)

		if stderr, err := runFFmpeg(cmd); err != nil {
			return fmt.Errorf("failed to extract frame %d: %w, stderr: %s", i, err, string(stderr))
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", filepath.Join(framesDir, "frame_%03d.jpg"),
		"-filter_complex", fmt.Sprintf("tile=%dx%d", 5, 2),
		safeImagePath,
	)

	if stderr, err := runFFmpeg(cmd); err != nil {
		return fmt.Errorf("failed to create sprite sheet: %w, stderr: %s", err, string(stderr))
	}

	vttContent := "WEBVTT\n\n"
	for i := 0; i < frameCount; i++ {
		startTime := float64(i) * interval
		endTime := startTime + interval
		x := (i % 5) * 160
		y := (i / 5) * 90

		vttContent += fmt.Sprintf("%02d:%02d:%02d.000 --> %02d:%02d:%02d.000\n",
			int(startTime)/3600, (int(startTime)%3600)/60, int(startTime)%60,
			int(endTime)/3600, (int(endTime)%3600)/60, int(endTime)%60)
		vttContent += fmt.Sprintf("%s#xywh=%d,%d,160,90\n\n", filepath.Base(safeImagePath), x, y)
	}

	if err := os.WriteFile(safeVTTPath, []byte(vttContent), 0644); err != nil {
		return fmt.Errorf("failed to write VTT file: %w", err)
	}

	return nil
}

func GenerateMasterPlaylist(outputPath string, qualities []QualityPlaylist, audioTracks []AudioPlaylist) error {

	safeOutput, err := sanitizePath(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	outputDir := filepath.Dir(safeOutput)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create playlist directory: %w", err)
	}

	content := "#EXTM3U\n#EXT-X-VERSION:3\n\n"

	for _, q := range qualities {
		content += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n", q.Bandwidth, q.Resolution)
		content += fmt.Sprintf("%s\n\n", q.RelativePath)
	}

	for _, a := range audioTracks {
		content += fmt.Sprintf("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",DEFAULT=%s,URI=\"%s\"\n",
			a.Label, boolToYesNo(a.Default), a.RelativePath)
	}

	if err := os.WriteFile(safeOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write master playlist: %w", err)
	}

	return nil
}

type QualityPlaylist struct {
	Quality      string
	Resolution   string
	Bandwidth    int
	RelativePath string
}

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

func CountSegments(dir string) int {
	files, err := filepath.Glob(filepath.Join(dir, "segment_*.ts"))
	if err != nil {
		return 0
	}
	return len(files)
}
