package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

	ffmpegThreadsOnce sync.Once
	ffmpegThreadsVal  int
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

// ffmpegThreads returns the thread count to pass to ffmpeg via -threads.
// Reads ENCODER_FFMPEG_THREADS once; 0 means "let ffmpeg choose" (default: 4).
func ffmpegThreads() int {
	ffmpegThreadsOnce.Do(func() {
		v := env.GetInt("ENCODER_FFMPEG_THREADS", 4)
		if v < 0 {
			v = 0
		}
		ffmpegThreadsVal = v
	})
	return ffmpegThreadsVal
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

// MultiQualitySpec describes one HLS rendition for TranscodeToHLSMultiQuality.
type MultiQualitySpec struct {
	Quality      string // e.g. "1080p" — used only for error messages
	Resolution   string // WxH
	VideoBitrate string // e.g. "5000k"
	OutputDir    string // absolute local directory for this rendition's segments
}

// TranscodeToHLSMultiQuality runs a single ffmpeg process that produces HLS
// renditions for every spec in one decode pass, dramatically reducing peak RAM
// usage compared to N separate TranscodeToHLS calls.
func TranscodeToHLSMultiQuality(ctx context.Context, inputPath string, specs []MultiQualitySpec, hasAudio bool) error {
	if len(specs) == 0 {
		return fmt.Errorf("no quality specs provided")
	}

	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeDirs := make([]string, len(specs))
	for i, spec := range specs {
		if !resolutionRe.MatchString(spec.Resolution) {
			return fmt.Errorf("invalid resolution token %q for quality %s", spec.Resolution, spec.Quality)
		}
		if !bitrateRe.MatchString(spec.VideoBitrate) {
			return fmt.Errorf("invalid bitrate token %q for quality %s", spec.VideoBitrate, spec.Quality)
		}
		sd, err := sanitizePath(spec.OutputDir)
		if err != nil {
			return fmt.Errorf("invalid output dir for quality %s: %w", spec.Quality, err)
		}
		if err := os.MkdirAll(sd, 0750); err != nil {
			return fmt.Errorf("failed to create output dir for quality %s: %w", spec.Quality, err)
		}
		safeDirs[i] = sd
	}

	n := len(specs)

	// Build filter_complex: split video (and audio if present) into N streams.
	videoLabels := make([]string, n)
	audioLabels := make([]string, n)
	for i := range specs {
		videoLabels[i] = fmt.Sprintf("[v%d]", i)
		audioLabels[i] = fmt.Sprintf("[a%d]", i)
	}

	filterComplex := fmt.Sprintf("[0:v]split=%d%s", n, strings.Join(videoLabels, ""))
	if hasAudio {
		filterComplex += fmt.Sprintf(";[0:a]asplit=%d%s", n, strings.Join(audioLabels, ""))
	}

	args := []string{
		"-nostdin",
	}
	if t := ffmpegThreads(); t > 0 {
		args = append(args, "-threads", strconv.Itoa(t))
	}
	args = append(args,
		"-i", safeInput,
		"-filter_complex", filterComplex,
	)

	for i, spec := range specs {
		args = append(args,
			"-map", videoLabels[i],
			"-c:v", "libx264",
			"-b:v", spec.VideoBitrate,
			"-s", spec.Resolution,
		)
		if hasAudio {
			args = append(args, "-map", audioLabels[i], "-c:a", "aac", "-b:a", "128k")
		} else {
			args = append(args, "-an")
		}
		args = append(args,
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(safeDirs[i], "segment_%03d.ts"),
			filepath.Join(safeDirs[i], "index.m3u8"),
		)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg multi-quality transcode failed: %w, stderr: %s", err, string(stderr))
	}
	return nil
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

	if err := os.MkdirAll(safeOutput, 0750); err != nil {
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

	if err := os.MkdirAll(safeOutput, 0750); err != nil {
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

// GenerateThumbnailsMulti extracts count thumbnails from the video in a single
// ffmpeg pass, evenly distributed across the video duration. Output files are
// named thumbnail_1.jpg through thumbnail_N.jpg in outputDir.
func GenerateThumbnailsMulti(ctx context.Context, inputPath, outputDir string, count int, duration float64) error {
	if count < 1 {
		return fmt.Errorf("count must be >= 1")
	}

	safeInput, err := sanitizePath(inputPath)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	safeDir, err := sanitizePath(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output dir: %w", err)
	}

	if err := os.MkdirAll(safeDir, 0750); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// fps=count/duration samples `count` frames evenly distributed.
	// Output filenames: thumbnail_1.jpg … thumbnail_N.jpg (%d is 1-based in ffmpeg).
	fpsExpr := fmt.Sprintf("%d/%.4f", count, duration)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-i", safeInput,
		"-vf", "fps="+fpsExpr,
		"-frames:v", strconv.Itoa(count),
		"-q:v", "2",
		filepath.Join(safeDir, "thumbnail_%d.jpg"),
	)

	stderr, err := runFFmpeg(cmd)
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnails generation failed: %w, stderr: %s", err, string(stderr))
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
	if err := os.MkdirAll(outputDir, 0750); err != nil {
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
	if err := os.MkdirAll(outputDir, 0750); err != nil {
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
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create sprite directory: %w", err)
	}

	const frameCount = 10
	interval := duration / float64(frameCount)

	// Single ffmpeg pass: sample frameCount frames evenly, scale to 160x90, tile 5x2.
	fpsExpr := fmt.Sprintf("%d/%.4f", frameCount, duration)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-i", safeInput,
		"-vf", fmt.Sprintf("fps=%s,scale=160:90,tile=5x2", fpsExpr),
		"-frames:v", "1",
		"-q:v", "2",
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

	if err := os.WriteFile(safeVTTPath, []byte(vttContent), 0600); err != nil {
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
	if err := os.MkdirAll(outputDir, 0750); err != nil {
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

	if err := os.WriteFile(safeOutput, []byte(content), 0600); err != nil {
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
