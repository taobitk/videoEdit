package server

import "velocityengine/internal/ffmpeg"

// Hooks returns ffmpeg.JobHooks bound to this StateManager.
func (sm *StateManager) Hooks() ffmpeg.JobHooks {
	return ffmpeg.JobHooks{
		SetPID: func(pid int) {
			sm.mu.Lock()
			sm.activePID = pid
			sm.mu.Unlock()
		},
		SetEncoder: func(activeEncoder, detail string) {
			sm.mu.Lock()
			sm.status.ActiveEncoder = activeEncoder
			sm.status.EncoderDetail = detail
			sm.broadcastLocked()
			sm.mu.Unlock()
		},
		ResetProgress: func() {
			sm.mu.Lock()
			sm.status.Progress = 0
			sm.status.FPS = 0
			sm.status.Speed = "0x"
			sm.status.TimeProcessed = "00:00:00"
			sm.status.ETA = "Calculating..."
			sm.broadcastLocked()
			sm.mu.Unlock()
		},
		UpdateProgress: func(snap ffmpeg.ProgressSnapshot) {
			sm.mu.Lock()
			if snap.Progress > sm.status.Progress || snap.Progress == 0 {
				sm.status.Progress = snap.Progress
			}
			if snap.FPS > 0 {
				sm.status.FPS = snap.FPS
			}
			if snap.Speed != "" {
				sm.status.Speed = snap.Speed
			}
			if snap.TimeProcessed != "" {
				sm.status.TimeProcessed = snap.TimeProcessed
			}
			if snap.ETA != "" {
				sm.status.ETA = snap.ETA
			}
			sm.broadcastLocked()
			sm.mu.Unlock()
		},
		Log: sm.AppendLog,
		Warn: func(msg string) {
			sm.mu.Lock()
			sm.status.Warnings = append(sm.status.Warnings, msg)
			sm.broadcastLocked()
			sm.mu.Unlock()
		},
	}
}
