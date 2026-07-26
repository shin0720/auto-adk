package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func handleWorkflowState(w http.ResponseWriter, r *http.Request) {
	root := getWorkspaceDir()
	if root == "" {
		root = uiProjectRoot
	}

	switch r.Method {
	case http.MethodGet:
		state, err := loadWorkflowState(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "workflow state load warning: %v\n", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	case http.MethodPost:
		var state workflowState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := saveWorkflowState(root, state); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleWorkflowEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type    string `json:"type"`
		AgentID string `json:"agentId"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uiWorkflowBroker.publish(req.Type, req.AgentID, workflowAgentName(req.AgentID), req.Message)
	w.WriteHeader(http.StatusNoContent)
}

func handleWorkflowCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	activeAgentCancelsMu.Lock()
	cancel, ok := activeAgentCancels[req.AgentID]
	activeAgentCancelsMu.Unlock()
	if ok {
		cancel()
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleWorkflowRunning(w http.ResponseWriter, r *http.Request) {
	activeAgentCancelsMu.Lock()
	running := make([]string, 0, len(activeAgentCancels))
	for id := range activeAgentCancels {
		running = append(running, id)
	}
	activeAgentCancelsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": running})
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	root := getWorkspaceDir()
	dst := filepath.Join(root, filepath.Base(header.Filename))
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "path": dst})
}

type providerStatus struct {
	ID        string `json:"id"`
	Connected bool   `json:"connected"`
	Version   string `json:"version,omitempty"`
	Issue     string `json:"issue,omitempty"`

	// PR #29: additive auth/availability hardening. Backward-compatible —
	// existing consumers keep reading `connected`. These fields never assert
	// that auto-run will succeed (no provider is executed to verify auth).
	AuthState         string `json:"authState,omitempty"`   // available | authRequired | manualOnly | unavailable
	StatusLabel       string `json:"statusLabel,omitempty"` // short human label
	StatusDetail      string `json:"statusDetail,omitempty"`
	IsPrimaryReviewer bool   `json:"isPrimaryReviewer"`
	IsOptionalSupport bool   `json:"isOptionalSupport"`
	CanAutoRun        bool   `json:"canAutoRun"`     // always false in PR #29 (no verified execution)
	CanManualImport   bool   `json:"canManualImport"`
}

// providerAuthConfigCandidates lists filesystem paths (relative to $HOME) that,
// if present, suggest the provider has local login/config. Only existence is
// checked via os.Stat — file CONTENTS and any secret/token values are NEVER read.
var providerAuthConfigCandidates = map[string][]string{
	"claude": {".claude", ".claude.json", ".config/claude"},
	"codex":  {".codex", ".codex.json", ".config/codex"},
	"gemini": {".gemini", ".gemini.json", ".config/gemini"},
}

// detectProviderConfig returns true if any known config path exists. It performs
// stat-only checks and never opens or reads the files.
func detectProviderConfig(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	for _, rel := range providerAuthConfigCandidates[name] {
		if _, statErr := os.Stat(filepath.Join(home, filepath.FromSlash(rel))); statErr == nil {
			return true
		}
	}
	return false
}

func handleProviderStatus(w http.ResponseWriter, r *http.Request) {
	names := []string{"claude", "codex", "gemini"}
	primary := map[string]bool{"claude": true, "codex": true}
	result := make([]providerStatus, 0, len(names))
	for _, name := range names {
		st := providerStatus{ID: name, CanManualImport: true, CanAutoRun: false}
		st.IsPrimaryReviewer = primary[name]
		st.IsOptionalSupport = !primary[name]
		if _, err := exec.LookPath(name); err != nil {
			// No binary: manual paste still works, auto-run does not.
			st.Connected = false
			st.AuthState = "unavailable"
			st.StatusLabel = "설치 안 됨"
			st.StatusDetail = "CLI 설치 미감지 · 수동 붙여넣기 가능"
			st.Issue = "CLI를 찾을 수 없습니다"
		} else if detectProviderConfig(name) {
			// Binary + local config detected. This is NOT a guarantee that a
			// real run would succeed — auto-run stays disabled until verified.
			st.Connected = true
			st.AuthState = "available"
			// Config detection is stat-only: a config file/dir exists, but auth is
			// NOT verified (no run happened). The label says so explicitly so users
			// do not read config-detection as a confirmed login.
			st.StatusLabel = "설정 감지됨 · 인증 미확인"
			st.StatusDetail = "설정 파일 감지 · 실행 검증 전(인증은 실제 실행 시 확인)"
		} else {
			// Binary present but no login/config detected → conservative.
			st.Connected = true
			st.AuthState = "authRequired"
			st.StatusLabel = "로그인 필요"
			st.StatusDetail = "로그인 설정 미감지 · 수동 사용 권장"
		}
		result = append(result, st)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func handleProviderConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch req.Provider {
	case "claude", "codex", "gemini":
		// valid provider names
	default:
		http.Error(w, "알 수 없는 provider입니다: "+req.Provider, http.StatusBadRequest)
		return
	}
	if _, err := exec.LookPath(req.Provider); err != nil {
		http.Error(w, req.Provider+" CLI를 찾을 수 없습니다. PATH에 설치되어 있는지 확인하세요.", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "detected",
		"provider": req.Provider,
		"message":  "CLI가 감지되었습니다. 인증이 필요한 경우 터미널에서 직접 진행하세요.",
	})
}
