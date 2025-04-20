package scraper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExtractVocals separates the vocal stem with Demucs, re‑encodes it to MP3
// and returns an *Audio describing the result.
func ExtractVocals(
	ctx context.Context,
	src *Audio,
	artifactDir string,
) (*Audio, error) {
	if src == nil {
		return nil, errors.New("input audio is nil")
	}
	if src.Path == "" {
		return nil, errors.New("input audio path is empty")
	}

	absArtifacts, err := filepath.Abs(artifactDir)
	if err != nil {
		return nil, fmt.Errorf("abs artifact dir: %w", err)
	}
	if err := os.MkdirAll(absArtifacts, fs.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir artifact dir: %w", err)
	}

	finalMP3 := filepath.Join(absArtifacts, "vocals.mp3")
	// Fast‑path: already done.
	if _, err := os.Stat(finalMP3); err == nil {
		dur, derr := Mp3DurationByFrames(finalMP3)
		if derr != nil {
			return nil, fmt.Errorf("calc duration: %w", derr)
		}
		return &Audio{Path: finalMP3, Duration: dur, Format: FormatMP3}, nil
	}

	/* ------------------------------------------------------------------
	   1. Run Demucs
	------------------------------------------------------------------ */

	separatedDir := filepath.Join(absArtifacts, "separated")
	if err := os.MkdirAll(separatedDir, fs.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir separated dir: %w", err)
	}

	fmt.Println("extracting vocals with Demucs …")

	demucsCmd := exec.CommandContext(
		ctx, "demucs",
		"--two-stems=vocals",
		"--out", separatedDir,
		src.Path,
	)
	demucsCmd.Stdout = os.Stdout
	demucsCmd.Stderr = os.Stderr

	if out, err := demucsCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("demucs: %w – %s", err, out)
	}

	/* ------------------------------------------------------------------
	   2. Locate the generated vocals.wav
	------------------------------------------------------------------ */

	base := strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path))
	vocalsWav := ""
	// Demucs pattern: separated/<model>/<basename>/vocals.wav
	if err := filepath.WalkDir(separatedDir, func(p string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "vocals.wav") && strings.HasSuffix(p, filepath.Join(base, "vocals.wav")) {
			vocalsWav = p
			return filepath.SkipDir // stop walking
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk separated dir: %w", err)
	}
	if vocalsWav == "" {
		return nil, fmt.Errorf("vocals.wav not found for %s", base)
	}

	/* ------------------------------------------------------------------
	   3. Convert WAV → MP3 (into temp file then rename)
	------------------------------------------------------------------ */

	tmpMP3, err := os.CreateTemp(absArtifacts, "vocals-*.tmp.mp3")
	if err != nil {
		return nil, fmt.Errorf("create temp mp3: %w", err)
	}
	tmpMP3.Close()
	defer os.Remove(tmpMP3.Name())

	ffmpegCmd := exec.CommandContext(
		ctx, "ffmpeg",
		"-y", // overwrite temp file if exists
		"-i", vocalsWav,
		"-acodec", "libmp3lame",
		"-q:a", "0",
		tmpMP3.Name(),
	)
	ffmpegCmd.Stdout = os.Stdout
	ffmpegCmd.Stderr = os.Stderr

	if out, err := ffmpegCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w – %s", err, out)
	}

	if err := os.Rename(tmpMP3.Name(), finalMP3); err != nil {
		return nil, fmt.Errorf("rename mp3: %w", err)
	}

	/* ------------------------------------------------------------------
	   4. Gather metadata
	------------------------------------------------------------------ */

	dur, err := Mp3DurationByFrames(finalMP3)
	if err != nil {
		return nil, fmt.Errorf("duration: %w", err)
	}

	return &Audio{
		Path:     finalMP3,
		Duration: dur,
		Format:   FormatMP3,
	}, nil
}
