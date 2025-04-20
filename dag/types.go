package dag

import (
	"context"
	"time"
)

type Artifacts map[string]string // key → absolute path on disk

type Task interface {
	ID() string
	Deps() []string
	Run(ctx context.Context, in Artifacts) (Artifacts, error)
	MaxRetries() uint64
	Timeout() time.Duration
	Cacheable() bool
}
