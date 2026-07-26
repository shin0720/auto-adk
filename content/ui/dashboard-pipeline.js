        async function runAgent(agentId, prompt, handoff, originalRequest = null) {
            lastPromptTokens = Math.ceil(prompt.length / 3.5);
            const agent = agentMap[agentId];
            setNodeStatus(agentId, 'active');
            appendTerminalLog(agent.name, handoff ? '이전 결과를 바탕으로 심층 분석 중...' : '분석 엔진을 가동합니다. 잠시만 기다려주세요...');
            const ns = getNodeState(agentId);
            ns.lastPrompt = prompt;
            ns.originalRequest = originalRequest !== null ? originalRequest : prompt;
            // Inject the wired-upstream hint once here so every agent run
            // (initial trigger, handoff, manual rerun, retry) is bounded to
            // the user's actual graph. Skip if the prompt already contains it
            // (handoff prompts may include it via buildWiredUpstreamHint).
            const wiredHint = buildWiredUpstreamHint(agentId);
            const promptWithHint = prompt.includes('[연결된 상류 작업자')
                ? prompt
                : prompt + '\n\n' + wiredHint;
            const effectivePrompt = systemPrompts.has(agentId)
                ? systemPrompts.get(agentId) + '\n\n---\n\n' + promptWithHint
                : promptWithHint;
            const res = await fetch('/api/workflow/run', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ agentId, prompt: effectivePrompt, context: Array.from(agentFiles.get(agentId) || []), handoff, providers: selectedProviders.get(agentId)?.size > 0 ? Array.from(selectedProviders.get(agentId)) : [agentDefaultProviders[agentId] || 'claude'] })
            });
            const data = await res.json();
            if (data.status === 'accepted') {
                // Async mode: result will arrive via SSE 'result' event.
                recoveryAttempts = 0;
                return;
            }
            if (data.status !== 'success') {
                setNodeStatus(agentId, 'error');
                // Guard: a response without a message must not render "undefined".
                const errMessage = (data && data.message) ? data.message : '알 수 없는 오류';
                appendTerminalLog('System', '❌ 에러: ' + errMessage);
                saveState();
                const recoveryTarget = errorRecoveryMap[agentId];
                if (recoveryTarget && recoveryAttempts < MAX_RECOVERY) {
                    recoveryAttempts++;
                    appendTerminalLog('System', `⚡ 에러 복구 루프 (${recoveryAttempts}/${MAX_RECOVERY}): ${recoveryTarget} 에이전트로 자동 재시도...`);
                    await runAgent(recoveryTarget, prompt, null);
                    return;
                }
                recoveryAttempts = 0;
                openDrawer(agent, prompt);
                return;
            }
            recoveryAttempts = 0;
            lastResult = data.result;
            const node = getNodeState(agentId);
            node.output = data.result;
            setNodeStatus(agentId, 'awaiting-approval');
            workflowState.approval = { pendingNodeId: agentId, lastDecision: '' };
            appendTerminalLog(agent.name, '결과 생성 완료. 승인 대기 상태로 전환합니다.');
            showPanel(agent.name + ' 분석 결과', data.result, agentId);
            saveState();
        }

        async function assignTask() {
            const prompt = document.getElementById('prompt-input').value.trim();
            if (!prompt || !selectedAgent) return;
            closeDrawer();
            recoveryAttempts = 0;
            const agentId = selectedAgent.id;
            const ns = getNodeState(agentId);
            const handoff = ns.rerunContext || null;
            ns.rerunContext = null;
            await runAgent(agentId, prompt, handoff);
        }

        // autoFixAgent re-runs a node that is in the error state using its last
        // recorded prompt. Falls back to opening the drawer when no prompt exists.
        async function autoFixAgent(agentId) {
            const ns = getNodeState(agentId);
            if (!ns.lastPrompt) {
                openDrawer(agentMap[agentId], '');
                return;
            }
            const agent = agentMap[agentId];
            appendTerminalLog('System', `🔄 [${agent ? agent.name : agentId}] 자동 재실행 시작...`);
            await runAgent(agentId, ns.lastPrompt, null, ns.originalRequest || null);
        }

        // nodeChecklists: Map<agentId, checklistItem[]>
        const nodeChecklists = new Map();

        function updateNodeChecklist(agentId, items) {
            nodeChecklists.set(agentId, items);
            renderNodeChecklist(agentId);
        }

        function renderNodeChecklist(agentId) {
            const items = nodeChecklists.get(agentId);
            const el = document.getElementById('node-' + agentId);
            if (!el) return;
            let box = el.querySelector('.checklist-box');
            if (!items || items.length === 0) {
                if (box) box.remove();
                return;
            }
            if (!box) {
                box = document.createElement('div');
                box.className = 'checklist-box';
                box.style.cssText = 'font-size:9px;margin-top:4px;text-align:left;max-height:80px;overflow-y:auto;background:rgba(0,0,0,.25);border-radius:4px;padding:3px 5px;';
                el.appendChild(box);
            }
            const done = items.filter(i => i.done).length;
            box.innerHTML = `<div style="color:var(--fg2);margin-bottom:2px;">${done}/${items.length} 완료</div>` +
                items.map(i => `<div style="color:${i.done ? 'var(--ok)' : 'var(--fg2)'}">${i.done ? '✅' : '⬜'} ${i.label}</div>`).join('');
        }

        function checklistAllDone(agentId) {
            const items = nodeChecklists.get(agentId);
            if (!items || items.length === 0) return true;
            return items.every(i => i.done);
        }

        function togglePipelineMode() {
            pipelineAutoMode = !pipelineAutoMode;
            localStorage.setItem('autopus-pipeline-mode', JSON.stringify(pipelineAutoMode));
            updatePipelineModeBtn();
            appendTerminalLog('System', pipelineAutoMode
                ? '🤖 자동 파이프라인 활성화 — 에이전트 완료 시 자동으로 다음 단계로 진행합니다.'
                : '🔍 수동 검사 모드 활성화 — 각 단계마다 결과를 검토하고 승인해야 합니다.');
        }

        function updatePipelineModeBtn() {
            const btn = document.getElementById('btn-pipeline-mode');
            if (!btn) return;
            if (pipelineAutoMode) {
                btn.textContent = '🤖 자동';
                btn.className = 'top-btn auto-on';
                btn.title = '자동 파이프라인 모드 (클릭하면 수동 검사로 전환)';
            } else {
                btn.textContent = '🔍 수동';
                btn.className = 'top-btn auto-off';
                btn.title = '수동 검사 모드 (클릭하면 자동으로 전환)';
            }
        }

        function getRootRequest(str) {
            const marker = '\n\n[이전 단계 완료:';
            const idx = str.indexOf(marker);
            const raw = idx !== -1 ? str.slice(0, idx) : str;
            const header = '[사용자 원래 요청]\n';
            return raw.startsWith(header) ? raw.slice(header.length).trim() : raw.trim();
        }

        function getAgentTaskInstruction(agentId, agent) {
            const tasks = {
                planner: '요구사항을 분석하고 구현 계획을 수립하세요. 어떤 파일을 만들고 어떤 기능을 구현할지 단계별로 명시하세요. 계획 문서를 실제 파일로 저장하세요.',
                spec: 'SPEC 문서 4종(spec.md, plan.md, acceptance.md, research.md)을 작성하거나 수정하세요. 기존 구현 코드가 있다면 누락된 SPEC을 보완하고 파일을 저장하세요.',
                arch: '시스템 아키텍처를 설계하고 테스트 인프라를 구성하세요. pytest.ini, conftest.py, 설정 파일 등 실제 코드 파일을 생성하고 구조를 보고하세요.',
                expl: '프로젝트 구조를 탐색하고 파일 목록, 주요 모듈, 의존성을 정리한 보고서를 작성하세요. 탐색 결과를 파일로 저장하세요.',
                exec: '백엔드 기능을 구현하세요. 실제 소스 파일을 작성하고 테스트를 통과시키세요. 생성된 파일 목록과 테스트 통과 결과(예: pytest 출력)를 반드시 보고하세요.',
                deep: '복잡한 작업을 체크포인트 방식으로 수행하세요. 각 단계마다 진행 상황을 보고하고 실제 파일을 생성하세요. 최종 결과물 목록과 검증 결과를 보고하세요.',
                dbug: '에러 메시지와 스택 트레이스를 분석하고 버그를 수정하세요. 수정한 파일명, 변경 내용, 수정 후 테스트 결과를 구체적으로 보고하세요.',
                anno: '@AX 태그를 스캔하고 코드 주석을 자동 적용하세요. 수정된 파일 목록과 적용된 태그 수를 보고하세요.',
                test: '단위, 통합, E2E 테스트를 작성하세요. 실제 테스트 파일을 생성하고 모두 통과시키세요. 커버리지 결과와 통과/실패 수를 보고하세요.',
                val: 'LSP 에러, 린트 경고, 테스트 통과 여부를 확인하세요. 실패 항목이 있으면 수정하고, 최종 통과 결과 요약을 보고하세요.',
                fend: '프론트엔드 페이지와 컴포넌트를 구현하세요. 실제 .tsx/.ts/.css 파일을 작성하고 빌드가 통과되는지 확인하세요. 생성된 파일 목록과 빌드 결과를 보고하세요.',
                uxv: '화면구조정의서 기준으로 프론트엔드 코드를 검증하고 UX 버그 보고서를 작성하세요. Critical/High/Medium 우선순위로 분류하고, 각 항목에 재현 방법과 수정 제안을 포함하세요.',
                perf: '벤치마크를 실행하고 성능 병목을 찾아 최적화하세요. 개선 전후 수치를 비교하고, 최적화된 코드와 측정 결과를 보고하세요.',
                rev: '코드 리뷰를 수행하세요. TRUST 5 기준(정확성·신뢰성·유용성·안전성·테스트)으로 구조적 문제, 보안 취약점, 테스트 누락 사항을 검토하고 구체적인 수정 제안을 보고하세요.',
                sec: 'OWASP Top 10 기준으로 보안 취약점을 검토하세요. SQL 인젝션, 인증 취약점, 민감정보 노출 등 실제 코드에서 취약점을 찾아 수정하고, 수정 내용과 검증 방법을 보고하세요.',
                devops: 'Write 도구로 backend/Dockerfile을 지금 즉시 생성하세요 (Python 3.11 + FastAPI + uvicorn 멀티스테이지 빌드). 그 다음 순서대로: frontend/Dockerfile (Node.js 20 + Next.js standalone), docker-compose.yml (services: db=PostgreSQL:15, cache=Redis:7, backend=포트8000, frontend=포트3000), .env.example (DATABASE_URL, REDIS_URL, ANTHROPIC_API_KEY, NEXT_PUBLIC_API_URL 포함), .github/workflows/ci.yml (push/PR 트리거, lint+test). 5개 파일 모두 Write 도구로 생성 완료 후 파일 목록을 출력하세요. Docker를 실제로 실행하지 마세요.'
            };
            return tasks[agentId] || ((agent ? agent.desc : agentId) + ' — 실제 파일이나 코드를 생성하고 결과물 목록을 보고하세요.');
        }

        // isReachableUpstream walks the user-wired connection graph BACKWARDS
        // from `fromAgent` to determine whether `targetUpstream` is part of
        // the wired pipeline that feeds into the current agent. This bounds
        // regression to the user's actual graph — if the user wired 5 nodes,
        // we can only regress within those 5 nodes. An agent cannot name
        // an arbitrary upstream that isn't wired to it.
        function isReachableUpstream(fromAgent, targetUpstream) {
            if (fromAgent === targetUpstream) return false;
            const visited = new Set([fromAgent]);
            const queue = [fromAgent];
            while (queue.length) {
                const current = queue.shift();
                const incoming = workflowState.connections.filter((c) => c.to === current);
                for (const c of incoming) {
                    if (c.from === targetUpstream) return true;
                    if (!visited.has(c.from)) {
                        visited.add(c.from);
                        queue.push(c.from);
                    }
                }
            }
            return false;
        }

        // listWiredUpstreams returns the names of agents that are reachable
        // upstream from `fromAgent` via the current connection graph. Used to
        // tell the user (and the agent) which upstream choices are valid.
        function listWiredUpstreams(fromAgent) {
            const visited = new Set();
            const queue = [fromAgent];
            while (queue.length) {
                const current = queue.shift();
                const incoming = workflowState.connections.filter((c) => c.to === current);
                for (const c of incoming) {
                    if (!visited.has(c.from)) {
                        visited.add(c.from);
                        queue.push(c.from);
                    }
                }
            }
            return Array.from(visited);
        }

        // buildWiredUpstreamHint returns a prompt fragment telling the agent
        // which upstream workers are reachable through the user's wired graph.
        // This bounds the agent's choices when reporting ## 상류 부족 보고.
        function buildWiredUpstreamHint(forAgentId) {
            const wired = listWiredUpstreams(forAgentId);
            if (!wired.length) {
                return '[연결된 상류 작업자]\n(없음 — 회귀 요청 불가. 본인 영역에서 결정하고 작업하거나, 정말 못하면 응답에 사유만 적으세요.)';
            }
            const lines = wired.map((id) => {
                const a = agentMap[id];
                return a ? `- ${id}: ${a.name} (${a.role})` : `- ${id}`;
            });
            return '[연결된 상류 작업자 — 회귀 시 이 중 하나만 명명 가능]\n' + lines.join('\n') +
                '\n위 목록 외의 ID 를 ## 상류 부족 보고 에 명명하면 회귀가 거부됩니다.';
        }

        // parseUpstreamReport scans an agent's output for the well-known
        // "## 상류 부족 보고" section. Returns null when not present.
        // Returns { upstreamId, missing } when the agent escalates to an
        // upstream worker that should be re-triggered.
        function parseUpstreamReport(output) {
            if (!output || typeof output !== 'string') return null;
            const m = output.match(/##\s*상류\s*부족\s*보고\s*\n([\s\S]*?)(?:\n##\s|$)/);
            if (!m) return null;
            const body = m[1];
            const agentMatch = body.match(/필요한\s*사전\s*작업자\s*[:：]\s*`?([a-z_][a-z0-9_]*)`?/i);
            if (!agentMatch) return null;
            const upstreamId = agentMatch[1].trim().toLowerCase();
            if (!agentMap[upstreamId]) return null;
            const missingMatch = body.match(/부족한\s*항목\s*[:：]\s*([\s\S]+?)(?=\n\n|\n##|$)/);
            return {
                upstreamId,
                missing: missingMatch ? missingMatch[1].trim() : '(이유 명시 없음)',
            };
        }

        // showPipelineCompleteModal renders a prominent "전체 파이프라인 완료"
        // dialog so the user can clearly see when all wired agents are done,
        // independent of the last individual agent's result panel.
