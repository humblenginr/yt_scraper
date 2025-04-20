package dag

import (
	"context"
	"sync"

	"github.com/cenkalti/backoff/v4"
)

type nodeState int

const (
	pending nodeState = iota
	running
	success
	failed
)

type dagEngine struct {
	nodes map[string]Task
	edges map[string][]string
	state map[string]nodeState
	errs  map[string]error
	mu    sync.RWMutex
	wg    sync.WaitGroup
}

func NewEngine(tasks []Task) *dagEngine {
	nodes := make(map[string]Task)
	edges := make(map[string][]string)
	state := make(map[string]nodeState)
	errs := make(map[string]error)
	for _, t := range tasks {
		nodes[t.ID()] = t
		edges[t.ID()] = t.Deps()
		state[t.ID()] = pending
	}
	return &dagEngine{nodes: nodes, edges: edges, state: state, errs: errs}
}

func (e *dagEngine) Run(ctx context.Context, rootCtxArtifacts Artifacts, workers int) error {
	sem := make(chan struct{}, workers)
	var rootMu sync.Mutex
	artifacts := rootCtxArtifacts

	for {
		executable := e.ready()
		if len(executable) == 0 {
			break
		}

		for _, id := range executable {
			sem <- struct{}{}
			e.wg.Add(1)
			go func(id string) {
				defer func() { <-sem; e.wg.Done() }()

				task := e.nodes[id]
				e.setState(id, running)

				// merge parent artifacts
				in := make(Artifacts)
				rootMu.Lock()
				for k, v := range artifacts {
					in[k] = v
				}
				rootMu.Unlock()

				// retry with backoff
				var out Artifacts
				operation := func() error {
					childCtx, cancel := context.WithTimeout(ctx, task.Timeout())
					defer cancel()

					var err error
					out, err = task.Run(childCtx, in)
					return err
				}

				b := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), task.MaxRetries())
				if err := backoff.Retry(operation, backoff.WithContext(b, ctx)); err != nil {
					e.setError(id, err)
					e.setState(id, failed)
					return
				}

				// persist child artifacts to shared map
				if out != nil {
					rootMu.Lock()
					for k, v := range out {
						artifacts[k] = v
					}
					rootMu.Unlock()
				}

				e.setState(id, success)
			}(id)
		}
	}

	e.wg.Wait()

	for _, err := range e.errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *dagEngine) ready() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var list []string
	for id, st := range e.state {
		if st != pending {
			continue
		}
		deps := e.edges[id]
		ok := true
		for _, d := range deps {
			if e.state[d] != success {
				ok = false
				break
			}
		}
		if ok {
			list = append(list, id)
		}
	}
	return list
}

func (e *dagEngine) setState(id string, s nodeState) {
	e.mu.Lock()
	e.state[id] = s
	e.mu.Unlock()
}

func (e *dagEngine) setError(id string, err error) {
	e.mu.Lock()
	e.errs[id] = err
	e.mu.Unlock()
}
