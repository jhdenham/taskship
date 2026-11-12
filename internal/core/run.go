package core

import "time"

// RunState is the state of an agent run.
type RunState string

const (
	RunStarting RunState = "starting" // recorded, agent not started yet
	RunRunning  RunState = "running"  // agent process started
	RunEnded    RunState = "ended"    // agent exited; ExitCode is set
	RunFailed   RunState = "failed"   // agent could not be started
)

// Run records one launch of an agent for a task on this machine.
type Run struct {
	ID            string     `json:"id"`
	TaskID        string     `json:"task_id"`
	TaskRef       string     `json:"task_ref"`
	Agent         string     `json:"agent"`
	Machine       string     `json:"machine"`
	WorkspacePath string     `json:"workspace_path"`
	SessionID     string     `json:"session_id,omitempty"`
	State         RunState   `json:"state"`
	ExitCode      *int       `json:"exit_code,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
}
