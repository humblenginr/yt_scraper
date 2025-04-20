package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type TimeAlignedWord struct {
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	Word            string  `json:"word"`
	ConfidenceScore float64 `json:"score"`
}

type TranscriptSegment struct {
	Text  string            `json:"text"`
	Start float64           `json:"start"`
	End   float64           `json:"end"`
	Words []TimeAlignedWord `json:"words"`
}

type TimeAlignedTranscript struct {
	Segments []TranscriptSegment `json:"segments"`
}

func Transcribe(vocals *Audio, artifactsDir string) (*TimeAlignedTranscript, error) {
	if !filepath.IsAbs(vocals.Path) {
		return nil, fmt.Errorf("vocals path: %s has to be absolute path", vocals.Path)
	}

	cmd := exec.Command("whisperx",
		vocals.Path,
		"--model", "large-v3",
		"--align_model", "WAV2VEC2_ASR_LARGE_LV60K_960H",
		"--batch_size", "4",
	)
	cmd.Dir = artifactsDir
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run whisperx: %v", err)
	}

	jsonData, err := os.ReadFile(filepath.Join(artifactsDir, "vocals.json"))
	if err != nil {
		log.Fatalf("Failed to read vocals.json: %v", err)
	}

	timeAlignedTranscript := &TimeAlignedTranscript{}
	if err := json.Unmarshal(jsonData, timeAlignedTranscript); err != nil {
		log.Fatalf("Failed to unmarshal vocals.json: %v", err)
	}

	return timeAlignedTranscript, nil
}
