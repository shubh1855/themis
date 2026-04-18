package worker

import (
	"context"
	"sync"
)

type Task struct {
	ID       string
	Type     string
	Status   string
	Progress float64
	Cancel   context.CancelFunc
}

// ProgressMsg goes to Bubbletea's global update
type ProgressMsg struct {
	TaskID   string
	Progress float64
	Status   string
}

type Pool struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

func NewPool() *Pool {
	return &Pool{
		tasks: make(map[string]*Task),
	}
}

// Dispatch starts a new goroutine for parallel stuff
func (p *Pool) Dispatch(id string, taskType string, sendUpdate func(msg interface{}), workerFunc func(context.Context, func(float64, string))) {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Task{
		ID:     id,
		Type:   taskType,
		Status: "Starting",
		Cancel: cancel,
	}

	p.mu.Lock()
	p.tasks[id] = t
	p.mu.Unlock()

	go func() {
		reportFunc := func(prog float64, status string) {
			p.mu.Lock()
			t.Progress = prog
			t.Status = status
			p.mu.Unlock()

			if sendUpdate != nil {
				sendUpdate(ProgressMsg{TaskID: id, Progress: prog, Status: status})
			}
		}

		workerFunc(ctx, reportFunc)
	}()
}

// ForEach allows iterating safely over current tasks
func (p *Pool) ForEach(fn func(id string, t *Task)) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for id, t := range p.tasks {
		fn(id, t)
	}
}

