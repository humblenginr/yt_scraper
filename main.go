package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/humblenginr/yt_rhymes_scraper/dag"
	"github.com/humblenginr/yt_rhymes_scraper/scraper"
)

const (
	ArtifactDirectory = "artifacts"
)

type DownloadTask struct {
	url     string
	outFile string
	retries uint
	timeout time.Duration
	cache   bool
}

func (t DownloadTask) ID() string             { return "download" }
func (t DownloadTask) Deps() []string         { return nil }
func (t DownloadTask) MaxRetries() uint       { return t.retries }
func (t DownloadTask) Timeout() time.Duration { return t.timeout }
func (t DownloadTask) Cacheable() bool        { return t.cache }
func (t DownloadTask) Run(ctx context.Context, _ dag.Artifacts) (dag.Artifacts, error) {
	if _, err := os.Stat(t.outFile); err == nil && t.cache {
		log.Printf("[download] cache hit -> %s", t.outFile)
		return dag.Artifacts{"audio": t.outFile}, nil
	}
	if _, err := scraper.DownloadYoutubeAudio(ctx, t.url, t.outFile); err != nil {
		return nil, err
	}
	return dag.Artifacts{"audio": t.outFile}, nil
}

type ExtractTask struct {
	retries uint
	timeout time.Duration
	cache   bool
}

func (t ExtractTask) ID() string             { return "extract" }
func (t ExtractTask) Deps() []string         { return []string{"download"} }
func (t ExtractTask) MaxRetries() uint       { return t.retries }
func (t ExtractTask) Timeout() time.Duration { return t.timeout }
func (t ExtractTask) Cacheable() bool        { return t.cache }
func (t ExtractTask) Run(ctx context.Context, in dag.Artifacts) (dag.Artifacts, error) {
	audio := in["audio"]
	dir := filepath.Dir(audio)
	out := filepath.Join(dir, "vocals.wav")

	if _, err := os.Stat(out); err == nil && t.cache {
		log.Printf("[extract] cache hit -> %s", out)
		return dag.Artifacts{"vocals": out}, nil
	}

	outPath, err := scraper.ExtractVocals(audio, dir)
	if err != nil {
		return nil, err
	}
	return dag.Artifacts{"vocals": outPath}, nil
}

type TranscribeTask struct {
	retries uint
	timeout time.Duration
	cache   bool
}

func (t TranscribeTask) ID() string             { return "transcribe" }
func (t TranscribeTask) Deps() []string         { return []string{"extract"} }
func (t TranscribeTask) MaxRetries() uint       { return t.retries }
func (t TranscribeTask) Timeout() time.Duration { return t.timeout }
func (t TranscribeTask) Cacheable() bool        { return t.cache }
func (t TranscribeTask) Run(ctx context.Context, in Artifacts) (Artifacts, error) {

	voc := in["vocals"]
	dir := filepath.Dir(voc)
	out := filepath.Join(dir, "transcript.json")

	if _, err := os.Stat(out); err == nil && t.cache {
		log.Printf("[transcribe] cache hit -> %s", out)
		return Artifacts{"transcript": out}, nil
	}

	jsonPath, err := scraper.Transcribe(voc, dir)
	if err != nil {
		return nil, err
	}
	return Artifacts{"transcript": jsonPath}, nil
}

type SegmentTask struct {
	retries uint
	timeout time.Duration
}

func (t SegmentTask) ID() string             { return "segment" }
func (t SegmentTask) Deps() []string         { return []string{"transcribe"} }
func (t SegmentTask) MaxRetries() uint       { return t.retries }
func (t SegmentTask) Timeout() time.Duration { return t.timeout }
func (t SegmentTask) Cacheable() bool        { return false }
func (t SegmentTask) Run(ctx context.Context, in Artifacts) (Artifacts, error) {

	voc := in["vocals"]
	tr := in["transcript"]

	_, err := scraper.Segment(voc, tr)
	return nil, err
}

func main() {
	artifactDirAbs, err := filepath.Abs(ArtifactDirectory)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	outPath := filepath.Join(artifactDirAbs, "audio.mp3")

	a, err := scraper.DownloadYoutubeAudio("https://www.youtube.com/watch?v=bQ8eDYWVzcU", outPath)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	vocals, err := scraper.ExtractVocals(a, artifactDirAbs)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	transcription, err := scraper.Transcribe(vocals, artifactDirAbs)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	segments, err := scraper.Segment(vocals, transcription)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
