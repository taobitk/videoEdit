package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"velocityengine/internal/model"
	"velocityengine/internal/syswin"
)

const maxLogLines = 200

// StateManager provides thread-safe engine status management and SSE notifications.
type StateManager struct {
	mu           sync.RWMutex
	status       model.EngineStatus
	activeCancel context.CancelFunc
	activePID    int
	jobDone      chan struct{}
	subs         map[chan struct{}]struct{}
}

// NewStateManager creates an idle StateManager.
func NewStateManager() *StateManager {
	return &StateManager{
		status: model.NewInitialStatus(),
		subs:   make(map[chan struct{}]struct{}),
	}
}

// Snapshot returns a thread-safe deep copy of the current status.
func (sm *StateManager) Snapshot() model.EngineStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.status.Clone()
}

// IsProcessing reports whether an active render/split/merge job is running.
func (sm *StateManager) IsProcessing() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.status.Status == "processing"
}

// Subscribe registers an SSE listener and returns a notification channel and unsubscribe func.
func (sm *StateManager) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	sm.mu.Lock()
	sm.subs[ch] = struct{}{}
	sm.mu.Unlock()

	return ch, func() {
		sm.mu.Lock()
		delete(sm.subs, ch)
		sm.mu.Unlock()
	}
}

func (sm *StateManager) broadcastLocked() {
	for ch := range sm.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// TryStartJob atomically transitions from non-processing to processing and creates a cancellable context.
func (sm *StateManager) TryStartJob(parentCtx context.Context, mode, initialStep, outFilename string) (context.Context, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.status.Status == "processing" {
		return nil, false
	}
	ctx, cancel := context.WithCancel(parentCtx)
	done := make(chan struct{})

	st := model.NewInitialStatus()
	st.Status = "processing"
	st.Mode = mode
	st.ETA = "Calculating..."
	st.CurrentStep = initialStep
	st.OutputFilename = outFilename

	sm.status = st
	sm.activeCancel = cancel
	sm.activePID = 0
	sm.jobDone = done
	sm.broadcastLocked()
	return ctx, true
}

// FinishJob transitions out of processing and closes the active job barrier.
func (sm *StateManager) FinishJob(err error, outFiles []model.OutputFile, outFilename, outURL, doneStep string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err != nil {
		sm.status.Status = "failed"
		sm.status.ErrorMessage = err.Error()
		sm.status.CurrentStep = "Failed."
		sm.status.OutputFilename = ""
		sm.status.OutputURL = ""
		sm.status.OutputFiles = []model.OutputFile{}
	} else {
		sm.status.Status = "completed"
		sm.status.Progress = 100.0
		sm.status.ETA = "0s"
		sm.status.CurrentStep = doneStep
		sm.status.OutputFilename = outFilename
		sm.status.OutputURL = outURL
		if outFiles == nil {
			outFiles = []model.OutputFile{}
		}
		sm.status.OutputFiles = outFiles
	}

	if sm.activeCancel != nil {
		sm.activeCancel()
		sm.activeCancel = nil
	}
	sm.activePID = 0
	if sm.jobDone != nil {
		close(sm.jobDone)
		sm.jobDone = nil
	}
	sm.broadcastLocked()
}

// CancelActiveJob cancels the running job context, kills its FFmpeg process tree, and waits briefly.
func (sm *StateManager) CancelActiveJob() bool {
	sm.mu.Lock()
	if sm.status.Status != "processing" {
		sm.mu.Unlock()
		return false
	}
	cancel := sm.activeCancel
	pid := sm.activePID
	done := sm.jobDone
	sm.status.CurrentStep = "Cancelling..."
	sm.broadcastLocked()
	sm.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if pid > 0 {
		_ = syswin.KillProcessTree(pid)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
	return true
}

// Reset clears engine state back to idle unless a job is actively running.
func (sm *StateManager) Reset() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.status.Status == "processing" {
		return fmt.Errorf("cannot reset while processing")
	}
	sm.status = model.NewInitialStatus()
	sm.broadcastLocked()
	return nil
}

// SetStep updates the current step text.
func (sm *StateManager) SetStep(step string) {
	sm.mu.Lock()
	sm.status.CurrentStep = step
	sm.broadcastLocked()
	sm.mu.Unlock()
}

// AppendLog adds a log line while bounding memory to maxLogLines.
func (sm *StateManager) AppendLog(line string) {
	sm.mu.Lock()
	sm.status.Logs = append(sm.status.Logs, line)
	if len(sm.status.Logs) > maxLogLines {
		sm.status.Logs = sm.status.Logs[len(sm.status.Logs)-maxLogLines:]
	}
	sm.broadcastLocked()
	sm.mu.Unlock()
}
