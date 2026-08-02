package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shin0720/auto-adk/pkg/config"
	"github.com/shin0720/auto-adk/pkg/orchestra"
)

var (
	activeAgentCancelsMu sync.Mutex
	activeAgentCancels   = map[string]context.CancelFunc{}
)

func registerAgentCancel(agentID string, cancel context.CancelFunc) {
	activeAgentCancelsMu.Lock()
	defer activeAgentCancelsMu.Unlock()
	activeAgentCancels[agentID] = cancel
}

func unregisterAgentCancel(agentID string) {
	activeAgentCancelsMu.Lock()
	defer activeAgentCancelsMu.Unlock()
	delete(activeAgentCancels, agentID)
}

func handleWorkflowRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Default-disabled guard. This legacy path spawns real provider subprocesses
	// (Claude with --dangerously-skip-permissions) and instructs direct file
	// edits, with no separate env gate of its own. Keep it structurally
	// unreachable unless explicitly opted in, so a stray "실전 분석 시작" click
	// cannot trigger a provider run by default. Must stay ahead of any binary
	// resolution or subprocess launch below.
	if !legacyWorkflowRunEnabled(os.Getenv) {
		writeLegacyWorkflowRunDisabled(w)
		return
	}

	var req workflowRunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	agentName := workflowAgentName(req.AgentID)

	// Capture workspace root once so all reads and the agent execution are consistent.
	// getWorkspaceDir reflects the latest handleWorkspaceChange navigation.
	projectRoot := getWorkspaceDir()
	if projectRoot == "" {
		projectRoot = uiProjectRoot
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		_ = json.NewEncoder(w).Encode(workflowRunResponse{Status: "error", Message: "Config load failed: " + err.Error()})
		return
	}

	// Append auto-context files specific to this agent (e.g. arch always gets 개발용/*.md).
	autoCtx := gatherAutoContextFiles(projectRoot, req.AgentID, req.Context)
	allContext := append(req.Context, autoCtx...)

	var ctxFiles strings.Builder
	for _, path := range allContext {
		data, _ := readWorkspaceFile(projectRoot, path)
		if len(data) == 0 {
			continue
		}
		ctxFiles.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n%s\n", path, string(data)))
	}

	prompt := req.Prompt
	if req.Handoff != nil {
		handoffOutput := req.Handoff.Output
		const maxHandoffRunes = 8000
		if runes := []rune(handoffOutput); len(runes) > maxHandoffRunes {
			handoffOutput = string(runes[:maxHandoffRunes]) + "\n...(이하 생략)"
		}
		prompt = fmt.Sprintf(
			"%s\n\n이전 에이전트 결과 요약:\n%s\n\n이전 에이전트 전체 출력:\n%s\n\n"+
				"---\n[즉시 실행 지시]\n"+
				"이전 단계의 결과를 이어받아 당신의 역할에 맞는 작업을 즉시 실행하라.\n"+
				"결정이 필요한 항목은 스스로 최선의 선택을 내리고 이유를 ## 결정 사항 에 기록하라.\n"+
				"Edit 도구로 파일을 직접 수정하고 ## 작업 요약 형식으로 보고한 뒤 종료하라.\n"+
				"사용자에게 질문하거나 선택지를 제시하지 마라.",
			req.Prompt,
			req.Handoff.Summary,
			handoffOutput,
		)
	}

	var providers []orchestra.ProviderConfig
	var providerErrors []string
	for name, provider := range cfg.Orchestra.Providers {
		resolvedBinary, err := resolveRunnableBinary(provider.Binary)
		if err != nil {
			providerErrors = append(providerErrors, fmt.Sprintf("%s(%s 없음)", name, provider.Binary))
			continue
		}
		// Claude requires --dangerously-skip-permissions in subprocess mode so
		// Write/Edit tool calls are not blocked waiting for interactive confirmation.
		args := append([]string(nil), provider.Args...)
		if strings.Contains(strings.ToLower(resolvedBinary), "claude") {
			// --print: non-interactive mode so Claude uses file tools and exits.
			// --dangerously-skip-permissions: allow Write/Edit without confirmation prompts.
			args = append(args, "--print", "--dangerously-skip-permissions")
		}
		providers = append(providers, orchestra.ProviderConfig{
			Name:          name,
			Binary:        resolvedBinary,
			Args:          args,
			PromptViaArgs: provider.PromptViaArgs,
		})
	}
	if len(req.Providers) > 0 {
		allowed := make(map[string]bool, len(req.Providers))
		for _, p := range req.Providers {
			allowed[p] = true
		}
		filtered := providers[:0]
		for _, p := range providers {
			if allowed[p.Name] {
				filtered = append(filtered, p)
			}
		}
		providers = filtered
	}

	if len(providers) == 0 {
		msg := "실행 가능한 CLI provider가 없습니다."
		if len(providerErrors) > 0 {
			msg += " 바이너리 미발견: " + strings.Join(providerErrors, ", ")
		}
		uiWorkflowBroker.publish("error", req.AgentID, agentName, msg)
		_ = json.NewEncoder(w).Encode(workflowRunResponse{Status: "error", Message: msg})
		return
	}

	// Dev agents get a checklist built from SPEC IDs found in context files.
	var checklist []checklistItem
	if devAgentIDs[req.AgentID] {
		checklist = buildSpecChecklist(projectRoot, req.Context)
	}
	if len(checklist) > 0 {
		prompt = buildDevPrompt(prompt, checklist)
		uiWorkflowBroker.publishChecklist(req.AgentID, agentName, checklist)
	}

	orchCfg := orchestra.OrchestraConfig{
		Prompt:         buildAgentPrompt(req.AgentID, agentName, prompt, ctxFiles.String()),
		Strategy:       orchestra.StrategyFastest,
		Providers:      providers,
		TimeoutSeconds: 1800,
		SubprocessMode: true,
	}

	startMsg := "분석 엔진을 가동합니다. 잠시만 기다려주세요..."
	if req.Handoff != nil {
		startMsg = "이전 결과를 바탕으로 심층 분석 중..."
	}
	uiWorkflowBroker.publish("started", req.AgentID, agentName, startMsg)

	// Return immediately so the HTTP connection is freed. Result arrives via SSE.
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(workflowRunResponse{Status: "accepted"})

	agentID := req.AgentID
	ctxFilePaths := req.Context
	initialChecklist := checklist
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		registerAgentCancel(agentID, cancel)
		defer func() {
			cancel()
			unregisterAgentCancel(agentID)
		}()

		const maxValidationRetries = 2

		var (
			cleanOutput string
			lastOutcome struct {
				result *orchestra.OrchestraResult
				err    error
			}
		)

		// Validation + retry loop. Re-run the agent up to maxValidationRetries times
		// if the response doesn't represent real work (e.g., ends with a question).
		for attempt := 0; attempt <= maxValidationRetries; attempt++ {
			type orchOutcome struct {
				result *orchestra.OrchestraResult
				err    error
			}
			done := make(chan orchOutcome, 1)
			cfgForRun := orchCfg
			go func() {
				r, e := orchestra.RunOrchestra(ctx, cfgForRun)
				done <- orchOutcome{r, e}
			}()

			ticker := time.NewTicker(30 * time.Second)
			startedAt := time.Now()
			var outcome orchOutcome
		wait:
			for {
				select {
				case outcome = <-done:
					break wait
				case <-ticker.C:
					elapsed := int(time.Since(startedAt).Minutes())
					uiWorkflowBroker.publish("working", agentID, agentName,
						fmt.Sprintf("⏳ 작업 중... (%d분 경과)", elapsed))
				case <-ctx.Done():
					ticker.Stop()
					uiWorkflowBroker.publish("error", agentID, agentName, "⛔ 작업이 취소되었습니다.")
					return
				}
			}
			ticker.Stop()

			lastOutcome.result = outcome.result
			lastOutcome.err = outcome.err

			if outcome.err != nil {
				uiWorkflowBroker.publish("error", agentID, agentName, outcome.err.Error())
				return
			}

			cleanOutput = outcome.result.Merged
			reason, ok := validateAgentOutput(agentID, cleanOutput)
			if ok {
				break
			}
			if attempt == maxValidationRetries {
				uiWorkflowBroker.publish("working", agentID, agentName,
					fmt.Sprintf("⚠️ 응답 검증 %d회 실패: %s. 결과를 그대로 표시합니다.", maxValidationRetries+1, reason))
				break
			}
			uiWorkflowBroker.publish("working", agentID, agentName,
				fmt.Sprintf("🔁 응답 검증 실패 (%s). 강화된 지시로 재시도합니다 (%d/%d)...", reason, attempt+1, maxValidationRetries))
			orchCfg.Prompt = buildRetryPrompt(orchCfg.Prompt, reason, attempt+1)
		}

		_ = lastOutcome

		if strings.TrimSpace(cleanOutput) != "" {
			uiWorkflowBroker.publish("chunk", agentID, agentName, cleanOutput)
		}

		// Update checklist based on ✅ SPEC-XXX 완료 markers in the output.
		finalChecklist := applyChecklistDone(initialChecklist, cleanOutput)
		if len(finalChecklist) > 0 {
			uiWorkflowBroker.publishChecklist(agentID, agentName, finalChecklist)
		}

		agentResult := &workflowAgentResult{
			Summary:      summarizeWorkflowOutput(cleanOutput),
			Output:       cleanOutput,
			FromAgent:    agentID,
			ContextFiles: ctxFilePaths,
			Timestamp:    time.Now().Format(time.RFC3339),
		}

		completedMsg := "결과 생성 완료"
		approvalMsg := "승인 대기 상태로 전환합니다."
		if len(finalChecklist) > 0 && !allChecklistDone(finalChecklist) {
			remaining := 0
			for _, it := range finalChecklist {
				if !it.Done {
					remaining++
				}
			}
			completedMsg = fmt.Sprintf("결과 생성 완료 (미완료 항목 %d개 남음)", remaining)
			approvalMsg = fmt.Sprintf("⚠️ 체크리스트 %d개 항목이 미완료입니다. 결과를 확인하고 승인 여부를 결정하세요.", remaining)
		}
		uiWorkflowBroker.publish("completed", agentID, agentName, completedMsg)
		uiWorkflowBroker.publish("awaiting_approval", agentID, agentName, approvalMsg)
		uiWorkflowBroker.publishWithResult("result", agentID, agentName, "결과 준비 완료", agentResult)

		_ = os.Stderr
	}()
}
