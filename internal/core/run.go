package core

import (
	"fmt"
	"time"
)

// RunState is the state of an agent run.
type RunState string

const (
	RunStarting RunState = "starting" // recorded, agent not started yet
	RunRunning  RunState = "running"  // agent process started
	RunEnded    RunState = "ended"    // agent exited; ExitCode is set
	RunFailed   RunState = "failed"   // agent could not be started
)

// RunMode is what an agent run is for.
type RunMode string

const (
	RunModeRun    RunMode = "run"    // work on the task, plan first
	RunModeRefine RunMode = "refine" // make the task ready to work on
)

// Run records one launch of an agent for a task on this machine.
type Run struct {
	ID            string     `json:"id"`
	Number        int        `json:"number"` // per task: #1, #2, ...
	TaskID        string     `json:"task_id"`
	TaskRef       string     `json:"task_ref"`
	Agent         string     `json:"agent"`
	Mode          RunMode    `json:"mode"`
	Machine       string     `json:"machine"`
	WorkspacePath string     `json:"workspace_path"`
	SessionID     string     `json:"session_id,omitempty"`
	State         RunState   `json:"state"`
	ExitCode      *int       `json:"exit_code,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
}

// RunRef names a run for display, e.g. "Claude (run #2)".
type RunRef struct {
	Agent  string
	Number int
}

func (r RunRef) String() string {
	return fmt.Sprintf("%s (run #%d)", AgentName(r.Agent), r.Number)
}

// AgentName returns an agent's display name, e.g. "Claude" for "claude".
func AgentName(agent string) string {
	if agent == "claude" {
		return "Claude"
	}
	return agent
}
