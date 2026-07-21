        // PR #53: real provider run RESULT SUMMARY (read-only, summary-only).
        //
        // Shows the outcome of the disabled-by-default endpoint
        // POST /api/planning-council/providers/run for ONE provider at a time,
        // triggered ONLY by an explicit user click (never on page load).
        //
        // Safety invariants (also asserted by a Go static-guard test):
        //   - NO rawText / error CONTENT is displayed or persisted. Only summary
        //     fields (status/executed/gate/exitCode/durationMs/truncated/
        //     cleanupOK/mutation count/authStateHint) are kept.
        //   - state.json therefore never receives raw provider output.
        //   - finalDecision is NEVER touched; canAutoRun stays false.
        //   - the legacy workflow-run path is NEVER called or reused.
        //   - one provider at a time via an in-flight guard.

        const PROVIDER_RUN_ENDPOINT = '/api/planning-council/providers/run';
        const PROVIDER_RUN_TIMEOUT_MS = 45000;

        // providerRunInFlight holds the single provider currently running. Only one
        // real run may be in flight at a time; a second click is ignored while set.
        let providerRunInFlight = '';

        function providerRunResultsList() {
            const c = councilState();
            if (!c.providerRunResults) c.providerRunResults = [];
            return c.providerRunResults;
        }

        function providerRunEntry(providerId) {
            const list = providerRunResultsList();
            let e = list.find(x => x.providerId === providerId);
            if (!e) {
                e = { providerId, source: 'real', status: '', executed: false,
                    gate: '', exitCode: 0, durationMs: 0, truncated: false,
                    cleanupOK: false, mutationCount: 0, authStateHint: '',
                    startedAt: '', finishedAt: '', ranAt: '' };
                list.push(e);
            }
            return e;
        }

        // summarizeProviderRunResponse copies ONLY safe summary fields from the
        // endpoint response. rawText and error CONTENT are intentionally dropped so
        // they can never be rendered or written to state.json.
        function summarizeProviderRunResponse(e, resp) {
            e.status = (resp && resp.status) || 'failed';
            e.executed = !!(resp && resp.executed);
            e.gate = (resp && resp.gate) || '';
            e.exitCode = (resp && typeof resp.exitCode === 'number') ? resp.exitCode : 0;
            e.durationMs = (resp && typeof resp.durationMs === 'number') ? resp.durationMs : 0;
            e.truncated = !!(resp && resp.truncated);
            e.cleanupOK = !!(resp && resp.cleanupOK);
            e.mutationCount = (resp && Array.isArray(resp.mutationViolations)) ? resp.mutationViolations.length : 0;
            e.authStateHint = (resp && resp.authStateHint) || '';
            e.startedAt = (resp && resp.startedAt) || '';
            e.finishedAt = (resp && resp.finishedAt) || '';
            e.ranAt = new Date().toISOString();
        }

        // runProviderRealResult performs ONE explicit, user-initiated run for a
        // single provider. It is only ever invoked from an onclick handler, never
        // on page load. The one-at-a-time guard blocks concurrent runs.
        async function runProviderRealResult(providerId) {
            if (providerRunInFlight) return;              // in-flight guard: one at a time
            providerRunInFlight = providerId;
            const e = providerRunEntry(providerId);
            e.status = 'running';
            renderIntakePanelIfOpen();
            let resp = null;
            try {
                const res = await fetch(PROVIDER_RUN_ENDPOINT, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: providerId,
                        prompt: 'Do not read or write any files. Do not inspect the repository. Reply with exactly one line: OK',
                        timeoutMs: PROVIDER_RUN_TIMEOUT_MS
                    })
                });
                resp = await res.json();
            } catch (_) {
                resp = { status: 'failed', executed: false, gate: '', cleanupOK: false };
            }
            summarizeProviderRunResponse(e, resp);
            providerRunInFlight = '';
            saveState();                                  // summary-only; no rawText / error content
            renderIntakePanelIfOpen();
        }

        // runResultKind maps the summary entry to a display kind. A repo mutation or
        // a failed cleanup on an executed run always dominates as BLOCKED.
        function runResultKind(e) {
            if (!e || !e.status) return 'idle';
            if (e.status === 'running') return 'running';
            if (e.mutationCount > 0) return 'blocked';
            if (e.executed === true && e.cleanupOK === false) return 'blocked';
            switch (e.status) {
                case 'completed': return 'completed';
                case 'authRequired': return 'action-needed';
                case 'failed': return 'failed';
                case 'timeout': return 'timeout';
                case 'unavailable': return 'unavailable';
                case 'disabled': return 'disabled';
                default: return 'failed';
            }
        }

        function runResultBadge(kind) {
            const cls = { idle: 'b-gray', running: 'b-amber', completed: 'b-green',
                'action-needed': 'b-amber', failed: 'b-red', timeout: 'b-red',
                unavailable: 'b-gray', disabled: 'b-gray', blocked: 'b-red' };
            const label = { idle: '실행 전', running: '실행 중', completed: '완료',
                'action-needed': '조치 필요 (authRequired)', failed: '실패', timeout: '타임아웃',
                unavailable: '미설치', disabled: '게이트 꺼짐', blocked: '차단 (BLOCKED)' };
            return `<span class="intake-badge ${cls[kind] || 'b-gray'}">${label[kind] || '실행 전'}</span>`;
        }

        function runResultDetail(e, kind) {
            if (kind === 'idle') {
                return `<p class="up-note">실제 Provider 실행은 게이트로 보호됩니다. 자동 실행은 꺼져 있습니다. 요약만 표시하며 원문 출력은 저장하지 않습니다.</p>`;
            }
            if (kind === 'running') {
                return `<p class="council-reason">실행 중… (실제 provider 1회, 한 번에 하나만).</p>`;
            }
            if (kind === 'disabled') {
                return `<p class="council-reason">실제 Provider 실행이 게이트로 비활성화되어 있습니다. 활성화하려면 AUTOPUS_PLANNING_COUNCIL_PROVIDER_RUN=1 이 필요합니다. 자동 실행은 계속 꺼져 있습니다.</p>`;
            }
            const fields = `status=${escC(e.status)} · executed=${e.executed} · gate=${escC(e.gate)}`
                + ` · exitCode=${e.exitCode} · durationMs=${e.durationMs} · truncated=${e.truncated}`
                + ` · cleanupOK=${e.cleanupOK} · mutations=${e.mutationCount}`
                + (e.authStateHint ? ` · authStateHint=${escC(e.authStateHint)}` : '');
            let note = '';
            if (kind === 'blocked') {
                note = `<p class="council-reason">⚠ BLOCKED: 저장소 변경(mutation) 또는 정리 실패(cleanupOK=false)가 감지되었습니다.</p>`;
            } else if (kind === 'action-needed') {
                note = `<p class="council-reason">authRequired — 실패가 아니라 인증 조치가 필요합니다. 수동 import로 대체할 수 있습니다.</p>`;
            } else if (kind === 'timeout') {
                note = `<p class="council-reason">타임아웃 — 예상 가능한 제한입니다. 원문은 저장하지 않습니다.</p>`;
            } else if (kind === 'unavailable') {
                note = `<p class="council-reason">provider CLI 미설치 — 수동 import 가능.</p>`;
            } else if (kind === 'failed') {
                note = `<p class="council-reason">실패 — 상세 원문/오류 내용은 표시·저장하지 않습니다 (요약만).</p>`;
            }
            return `<p class="council-reason">${fields}</p>${note}`;
        }

        function runResultCardHtml(participant) {
            const id = participant.id, name = participant.name || id;
            const e = providerRunResultsList().find(x => x.providerId === id) || { providerId: id, status: '' };
            const kind = runResultKind(e);
            const disabledAttr = providerRunInFlight ? 'disabled' : '';
            const btn = `<button class="intake-btn active" ${disabledAttr} onclick="runProviderRealResult('${id}')">실제 실행 (1회)</button>`;
            return `<div class="mock-card"><div class="up-row"><strong>${escC(name)}</strong>${runResultBadge(kind)}</div>`
                + `<div class="q-toolbar mock-toolbar">${btn}</div>`
                + runResultDetail(e, kind)
                + `</div>`;
        }

        // renderProviderRunResult returns the real-run summary section. Participants
        // come from the council snapshot when present, else the static list.
        function renderProviderRunResult() {
            const c = councilState();
            const parts = (c.participants && c.participants.length)
                ? c.participants
                : COUNCIL_PARTICIPANTS.map(p => ({ id: p.id, name: p.name }));
            const cards = parts.map(p => runResultCardHtml(p)).join('');
            return `<div class="mock-block">
                <h4 class="q-subtitle">실제 실행 결과 요약 (Real · summary-only)</h4>
                <p class="up-note">🔒 명시 클릭으로 한 번에 하나만 실행합니다. 원문/오류 원문은 표시·저장하지 않습니다. finalDecision 과 자동 실행은 변경되지 않습니다.</p>
                ${cards}
            </div>`;
        }
