package audio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrFFmpegNotFound = fmt.Errorf("ffmpeg not found in PATH; install ffmpeg to enable audio conversion")

// ConvertToOpus converts audio data (any ffmpeg-supported format) to OGG/Opus.
// Returns the opus-encoded bytes and the duration in milliseconds.
// Requires ffmpeg installed on the host.
func ConvertToOpus(ctx context.Context, data []byte, srcFilename string) (opusData []byte, durationMs int64, err error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, 0, ErrFFmpegNotFound
	}

	tmpDir, err := os.MkdirTemp("", "audio-convert-*")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ext := filepath.Ext(srcFilename)
	if ext == "" {
		ext = ".bin"
	}
	inPath := filepath.Join(tmpDir, "input"+ext)
	outPath := filepath.Join(tmpDir, "output.ogg")

	if err := os.WriteFile(inPath, data, 0o600); err != nil {
		return nil, 0, fmt.Errorf("failed to write input file: %w", err)
	}

	// -ac 1: mono (voice), -ar 48000: Opus standard sample rate
	// -b:a 48k: good quality for voice, -vn: strip video/cover art
	args := []string{
		"-i", inPath,
		"-vn",
		"-c:a", "libopus",
		"-ac", "1",
		"-ar", "48000",
		"-b:a", "48k",
		"-y", outPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("ffmpeg conversion failed: %s", strings.TrimSpace(stderr.String()))
	}

	opusData, err = os.ReadFile(outPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read output file: %w", err)
	}

	durationMs = DetectDurationMs(opusData, "audio/ogg")
	if durationMs <= 0 {
		durationMs = DetectDurationMs(data, guessMimeFromExt(ext))
	}

	return opusData, durationMs, nil
}

func guessMimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp3":
		return "audio/mpeg"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	default:
		return "audio/unknown"
	}
}
