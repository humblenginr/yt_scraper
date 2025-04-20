package scraper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// DownloadYoutubeAudio downloads the best‑quality audio track of a YouTube
// video, converts it to MP3 (via yt‑dlp + ffmpeg) and returns metadata.
//
// The caller controls cancellation with ctx.  The function is idempotent:
// if outputPath already exists it simply calculates duration and returns.
func DownloadYoutubeAudio(
	ctx context.Context,
	videoURL string,
	outputPath string,
) (*Audio, error) {
	if videoURL == "" {
		return nil, errors.New("videoURL cannot be empty")
	}

	// Resolve absolute path and create parent directory.
	absOut, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("make abs path: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(absOut), fs.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir output dir: %w", err)
	}

	// Fast‑path: file already present.
	if _, err := os.Stat(absOut); err == nil {
		dur, derr := Mp3DurationByFrames(absOut)
		if derr != nil {
			return nil, fmt.Errorf("calc duration: %w", derr)
		}
		return &Audio{Path: absOut, Duration: dur, Format: FormatMP3}, nil
	}

	// Download to a temp file then atomically rename.
	tmp, err := os.CreateTemp(filepath.Dir(absOut), "*.mp3.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	_ = tmp.Close()
	defer os.Remove(tmp.Name())

	args := []string{
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--output", tmp.Name(),
		videoURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w – %s", err, out)
	}

	// Move temp → final.
	if err := os.Rename(tmp.Name(), absOut); err != nil {
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	dur, err := Mp3DurationByFrames(absOut)
	if err != nil {
		return nil, fmt.Errorf("calc duration: %w", err)
	}

	return &Audio{
		Path:     absOut,
		Duration: dur,
		Format:   FormatMP3,
	}, nil
}
