package scraper

import (
	"io"
	"os"
	"time"

	"github.com/tcolgate/mp3"
)

func Mp3DurationByFrames(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	d := mp3.NewDecoder(f)
	var (
		frame   mp3.Frame
		skipped int
		total   time.Duration
	)
	for {
		if err := d.Decode(&frame, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		total += frame.Duration()
	}
	return total, nil
}
