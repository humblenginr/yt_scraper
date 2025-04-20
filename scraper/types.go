package scraper

import "time"

type Format string

const (
	FormatMP3 Format = "mp3"
	FormatM4A Format = "m4a"
	FormatWAV Format = "wav"
)

type Audio struct {
	Path     string        `json:"path"`
	Duration time.Duration `json:"duration"`
	Format   Format        `json:"format"`
}

type AudioWithTranscript struct {
	Audio
	Text string `json:"text"`
}
