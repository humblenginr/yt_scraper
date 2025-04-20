package scraper

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	// confidenceThreshold: Below this threshold, we assume that the whisperx transcription is not reliable
	confidenceThreshold = 0.60
	// consecutiveLowConfidenceThreshold: If there are more than this number of consecutive low confidence words, we skip the segment
	consecutiveLowConfidenceThreshold = 3
	// percentageLowConfidenceThresholdWithinSegment: If more than this percentage of words have low confidence, we skip the segment
	percentageLowConfidenceThresholdWithinSegment = 0.50
	// minSegmentDurationSeconds: Skip segments shorter than this duration
	minSegmentDurationSeconds = 2.0
	// outputSegmentFormat: The audio format for the output segments
	outputSegmentFormat = FormatWAV // Assuming WAV output, adjust if needed
	outputSegmentExt    = ".wav"    // File extension for the output format
)

func Segment(audio *Audio, tat *TimeAlignedTranscript) ([]AudioWithTranscript, error) {
	// Create an output directory for the segments
	// Use the directory of the input audio file
	baseName := filepath.Base(audio.Path)
	ext := filepath.Ext(baseName)
	stem := baseName[:len(baseName)-len(ext)]
	outputDir := filepath.Join(filepath.Dir(audio.Path), fmt.Sprintf("%s_segments", stem))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating output directory %s: %v. Skipping segmentation.", outputDir, err)
	}
	log.Printf("Splitting audio file: %s", audio.Path)
	log.Printf("Output directory: %s", outputDir)

	var resultSegments []AudioWithTranscript
	for i, seg := range tat.Segments {
		segmentDuration := seg.End - seg.Start

		// Skip segments that are too short
		if segmentDuration < minSegmentDurationSeconds {
			log.Printf("Warning: Segment %d is too short (%.2fs < %.2fs). Skipping.", i, segmentDuration, minSegmentDurationSeconds)
			continue
		}

		// Skip segments with invalid start/end times
		if seg.Start >= seg.End || seg.Start < 0 {
			log.Printf("Warning: Segment %d has invalid start/end times (start: %.2fs, end: %.2fs). Skipping.", i, seg.Start, seg.End)
			continue
		}

		// Analyze word confidence if words are present
		if len(seg.Words) > 0 {
			lowConfidenceCount := 0
			consecutiveLowConfidence := 0
			maxConsecutiveLowConfidence := 0

			for _, word := range seg.Words {
				if word.ConfidenceScore < confidenceThreshold {
					lowConfidenceCount++
					consecutiveLowConfidence++
				} else {
					if consecutiveLowConfidence > maxConsecutiveLowConfidence {
						maxConsecutiveLowConfidence = consecutiveLowConfidence
					}
					consecutiveLowConfidence = 0 // Reset counter
				}
			}
			// Check after the loop for the last sequence
			if consecutiveLowConfidence > maxConsecutiveLowConfidence {
				maxConsecutiveLowConfidence = consecutiveLowConfidence
			}

			// Skip if more than threshold percentage of words have low confidence
			percentageLowConfidence := float64(lowConfidenceCount) / float64(len(seg.Words))
			if percentageLowConfidence > percentageLowConfidenceThresholdWithinSegment {
				log.Printf("Warning: Segment %d has >%.0f%% low confidence words (%.2f%%). Skipping.", i, percentageLowConfidenceThresholdWithinSegment*100, percentageLowConfidence*100)
				continue
			}

			// Skip if there are threshold+ consecutive low confidence words
			if maxConsecutiveLowConfidence >= consecutiveLowConfidenceThreshold {
				log.Printf("Warning: Segment %d has %d+ consecutive low confidence words (%d found). Skipping.", i, consecutiveLowConfidenceThreshold, maxConsecutiveLowConfidence)
				continue
			}
		}

		// Construct output filename for audio segment
		// Using index and times for uniqueness
		segmentBase := fmt.Sprintf("segment_%03d_%.2fs_%.2fs", i, seg.Start, seg.End)
		audioFilename := segmentBase + outputSegmentExt
		outputPath := filepath.Join(outputDir, audioFilename)

		// Use ffmpeg to extract the audio segment
		// ffmpeg -i <input> -ss <start> -to <end> <output>
		// Using -to specifies the absolute end time.
		// Omitting -c copy to ensure output is in the desired format (WAV by default)
		cmd := exec.Command("ffmpeg",
			"-i", audio.Path,
			"-ss", fmt.Sprintf("%f", seg.Start),
			"-to", fmt.Sprintf("%f", seg.End),
			// "-c", "copy", // Remove this to re-encode to WAV (or desired format)
			outputPath,
		)
		cmd.Stderr = os.Stderr // Redirect ffmpeg stderr for debugging

		log.Printf("Running command: %s", cmd.String())
		if err := cmd.Run(); err != nil {
			log.Printf("Error running ffmpeg for segment %d (start: %.2f, end: %.2f): %v. Skipping.", i, seg.Start, seg.End, err)
			continue // Skip this segment if ffmpeg fails
		}

		// Get absolute path for consistency
		absOutputPath, err := filepath.Abs(outputPath)
		if err != nil {
			return nil, fmt.Errorf("warning: could not get absolute path for %s: %v. Using relative path.", outputPath, err)
		}

		// Append successful segment info to results
		resultSegments = append(resultSegments, AudioWithTranscript{
			Audio: Audio{
				Path:     absOutputPath,
				Duration: time.Duration(segmentDuration * float64(time.Second)),
				Format:   outputSegmentFormat,
			},
			Text: seg.Text,
		})
	}

	log.Printf("Successfully split audio into %d segments in %s", len(resultSegments), outputDir)
	return resultSegments, nil
}
