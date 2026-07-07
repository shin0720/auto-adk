package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	workflowStateVersion = 2
	workflowStateFile    = ".autopus/workflows/state.json"
)

type workflowState struct {
	Version       int                  `json:"version"`
	LastUpdated   string               `json:"lastUpdated,omitempty"`
	Nodes         []workflowNodeState  `json:"nodes"`
	Connections   []workflowConnection `json:"connections"`
	Logs          []workflowLogEntry   `json:"logs"`
	Approval      workflowApproval     `json:"approval"`
	SystemPrompts map[string]string    `json:"systemPrompts,omitempty"`

	// Planner intake / readiness / design shells (PR #23). The server does not
	// interpret these; it only round-trips the client-authored JSON so the
	// dashboard can persist planning metadata. json.RawMessage keeps arbitrary
	// nested shapes intact, and omitempty means legacy state.json files without
	// these keys decode without error.
	Intake             json.RawMessage `json:"intake,omitempty"`
	Readiness          json.RawMessage `json:"readiness,omitempty"`
	DesignBrief        json.RawMessage `json:"designBrief,omitempty"`
	UiuxGate           json.RawMessage `json:"uiuxGate,omitempty"`
	AcceptanceEvidence json.RawMessage `json:"acceptanceEvidence,omitempty"`
	CostQuota          json.RawMessage `json:"costQuota,omitempty"`
	Maintenance        json.RawMessage `json:"maintenance,omitempty"`
	Rollback           json.RawMessage `json:"rollback,omitempty"`
	UiuxDiscovery      json.RawMessage `json:"uiuxDiscovery,omitempty"`

	// Manual Planning Council + finalDecision (PR #26a). Same RawMessage
	// passthrough pattern: the server only round-trips the client-authored
	// JSON, never interpreting it. omitempty keeps legacy state.json files
	// (without these keys) decoding without error.
	PlanningCouncil json.RawMessage `json:"planningCouncil,omitempty"`
	FinalDecision   json.RawMessage `json:"finalDecision,omitempty"`
}

type checklistItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Done  bool   `json:"done"`
}

type workflowNodeState struct {
	ID              string               `json:"id"`
	X               string               `json:"x,omitempty"`
	Y               string               `json:"y,omitempty"`
	Status          string               `json:"status,omitempty"`
	Output          *workflowAgentResult `json:"output,omitempty"`
	LastPrompt      string               `json:"lastPrompt,omitempty"`
	OriginalRequest string               `json:"originalRequest,omitempty"`
	Checklist       []checklistItem      `json:"checklist,omitempty"`
}

type workflowConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type workflowLogEntry struct {
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
	Agent     string `json:"agent"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type workflowApproval struct {
	PendingNodeID string `json:"pendingNodeId,omitempty"`
	LastDecision  string `json:"lastDecision,omitempty"`
}

type workflowAgentResult struct {
	Summary      string   `json:"summary"`
	Output       string   `json:"output"`
	FromAgent    string   `json:"fromAgent"`
	ContextFiles []string `json:"contextFiles,omitempty"`
	Timestamp    string   `json:"timestamp"`
}

type workflowRunRequest struct {
	AgentID   string               `json:"agentId"`
	Prompt    string               `json:"prompt"`
	Context   []string             `json:"context"`
	Handoff   *workflowAgentResult `json:"handoff,omitempty"`
	Providers []string             `json:"providers,omitempty"`
}

type workflowRunResponse struct {
	Status  string               `json:"status"`
	Message string               `json:"message,omitempty"`
	Result  *workflowAgentResult `json:"result,omitempty"`
}

type workflowStreamEvent struct {
	ID        string               `json:"id"`
	Type      string               `json:"type"`
	AgentID   string               `json:"agentId,omitempty"`
	AgentName string               `json:"agentName,omitempty"`
	Message   string               `json:"message"`
	Timestamp string               `json:"timestamp"`
	Result    *workflowAgentResult `json:"result,omitempty"`
	Checklist []checklistItem      `json:"checklist,omitempty"`
}

func newWorkflowState() workflowState {
	return workflowState{
		Version:       workflowStateVersion,
		Nodes:         []workflowNodeState{},
		Connections:   []workflowConnection{},
		Logs:          []workflowLogEntry{},
		Approval:      workflowApproval{},
		SystemPrompts: map[string]string{},
	}
}

func workflowStatePath(root string) string {
	return filepath.Join(root, filepath.FromSlash(workflowStateFile))
}

func loadWorkflowState(root string) (workflowState, error) {
	return loadWorkflowStateFromPath(workflowStatePath(root))
}

func loadWorkflowStateFromPath(path string) (workflowState, error) {
	state := newWorkflowState()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return newWorkflowState(), err
	}
	if state.Version == 0 {
		state.Version = workflowStateVersion
	}
	if state.Nodes == nil {
		state.Nodes = []workflowNodeState{}
	}
	if state.Connections == nil {
		state.Connections = []workflowConnection{}
	}
	if state.Logs == nil {
		state.Logs = []workflowLogEntry{}
	}
	return state, nil
}

func saveWorkflowState(root string, state workflowState) error {
	return saveWorkflowStateToPath(workflowStatePath(root), state)
}

func saveWorkflowStateToPath(path string, state workflowState) error {
	state.Version = workflowStateVersion
	state.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	if state.Nodes == nil {
		state.Nodes = []workflowNodeState{}
	}
	if state.Connections == nil {
		state.Connections = []workflowConnection{}
	}
	if state.Logs == nil {
		state.Logs = []workflowLogEntry{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func summarizeWorkflowOutput(output string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(output)), " ")
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) <= 160 {
		return normalized
	}
	return string(runes[:160]) + "..."
}
