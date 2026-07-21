        // PR #26a: Manual Planning Council section. Provider availability is a
        // read-only snapshot; opinions are typed by hand. There is NO provider
        // execution, NO /api/workflow/run call, NO auth, NO backend endpoint.

        // Seed default so the shell renders before the first loadState().
        (function seedPlanningCouncilDefault() {
            if (!workflowState.planningCouncil) workflowState.planningCouncil = defaultPlanningCouncil();
        })();

        // Restore from server state; a missing key falls back to the default so
        // legacy state.json files load without error.
        function restorePlanningCouncilState(data) {
            workflowState.planningCouncil = (data && data.planningCouncil) || defaultPlanningCouncil();
        }

        function councilState() {
            if (!workflowState.planningCouncil) workflowState.planningCouncil = defaultPlanningCouncil();
            return workflowState.planningCouncil;
        }

        // generateCouncilSnapshot captures the input context and refreshes the
        // participant availability list from the read-only provider status API.
        // On fetch failure the UI stays usable: every participant falls back to
        // manualOnly and nothing breaks.
        async function generateCouncilSnapshot() {
            const c = councilState();
            c.inputSnapshot = buildCouncilInputSnapshot();
            if (!c.createdAt) c.createdAt = new Date().toISOString();
            const priorNotes = c.providerNotes || [];
            const noteFor = (id) => (priorNotes.find(n => n.providerId === id) || {}).note || '';
            let statuses = null;
            try {
                const res = await fetch('/api/providers/status');
                if (res.ok) statuses = await res.json();
            } catch (_) { statuses = null; }
            const unavailable = [];
            // PR #29: map the hardened authState instead of trusting connected.
            // authRequired stays usable via manual import (manualOnly, non-blocking);
            // only a missing binary is treated as unavailable.
            const authToStatus = { available: 'available', authRequired: 'manualOnly', manualOnly: 'manualOnly', unavailable: 'unavailable' };
            const participants = COUNCIL_PARTICIPANTS.map(p => {
                let status = 'manualOnly';
                let issue = '';
                if (Array.isArray(statuses)) {
                    const st = statuses.find(s => s.id === p.id);
                    if (st && st.authState) {
                        status = authToStatus[st.authState] || 'manualOnly';
                        if (status !== 'available') issue = st.statusDetail || st.statusLabel || st.issue || '';
                    } else if (st && st.connected) {
                        status = 'available';
                    } else {
                        status = 'unavailable';
                        issue = (st && (st.statusDetail || st.issue)) || 'CLI를 찾을 수 없습니다';
                    }
                    if (status === 'unavailable') unavailable.push({ id: p.id, reason: issue });
                } else {
                    issue = 'provider status를 불러오지 못해 수동 입력만 가능합니다';
                }
                return { id: p.id, name: p.name, status, available: status === 'available', issue };
            });
            c.participants = participants;
            c.unavailableProviders = unavailable;
            c.providerNotes = COUNCIL_PARTICIPANTS.map(p => ({ providerId: p.id, note: noteFor(p.id), source: 'manual' }));
            c.status = 'snapshot_ready';
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(Array.isArray(statuses) ? '참여자 가용성 스냅샷을 갱신했습니다.' : 'provider status 미확인 — 전원 manualOnly로 표시합니다.');
        }

        function onProviderNoteInput(providerId, value) {
            const c = councilState();
            if (!c.providerNotes) c.providerNotes = [];
            let entry = c.providerNotes.find(n => n.providerId === providerId);
            if (!entry) { entry = { providerId, note: '', source: 'manual' }; c.providerNotes.push(entry); }
            entry.note = value;
        }

        // List-style fields (conflicts/risks/...) are edited as one item per
        // line, matching the existing rejectedStyles pattern. This supports
        // add (new line), edit (change line), delete (remove line) compactly.
        function onCouncilListInput(field, el) {
            councilState()[field] = el.value.split('\n').map(s => s.trim()).filter(Boolean);
        }

        function onCouncilConsensusInput(el) {
            councilState().consensus = el.value;
        }

        function saveCouncil() {
            saveState();
            intakeToast('Planning Council 내용이 저장되었습니다.');
        }

        function councilBadge(status) {
            const map = { available: 'b-green', unavailable: 'b-gray', skipped: 'b-gray', failed: 'b-red', manualOnly: 'b-amber' };
            return `<span class="intake-badge ${map[status] || 'b-gray'}">${status}</span>`;
        }

        function participantCardHtml(p, note) {
            const reason = p.issue ? `<p class="council-reason">사유: ${escC(p.issue)}</p>` : '';
            return `<div class="council-participant"><div class="up-row"><strong>${escC(p.name)}</strong>${councilBadge(p.status)}</div>${reason}`
                + `<textarea class="q-input" rows="2" placeholder="이 AI의 의견을 직접 붙여넣거나 입력하세요 (수동 입력)" oninput="onProviderNoteInput('${p.id}', this.value)">${escC(note)}</textarea></div>`;
        }

        function councilListField(label, field, ph) {
            const val = (councilState()[field] || []).join('\n');
            return `<h4 class="q-subtitle">${label}</h4><textarea class="q-input" rows="2" placeholder="${ph}" oninput="onCouncilListInput('${field}', this)">${escC(val)}</textarea>`;
        }

        function councilSnapshotSummary(snap) {
            if (!snap) return intakeEmpty('아직 스냅샷이 없습니다. "스냅샷 생성"을 눌러 기획 컨텍스트를 캡처하세요.');
            const req = snap.requiredAnswered ? '필수 답변 완료' : '필수 답변 미완료';
            return `<div class="council-snap"><p><strong>요청:</strong> ${escC(snap.originalRequest || '(비어 있음)')}</p>`
                + `<p><strong>상태:</strong> ${req} · 준비물 ${snap.userPrepChecklist.length}개 · 참고 사이트 ${snap.referenceSites.length} · 참고 이미지 ${snap.referenceImages.length}</p></div>`;
        }

        // renderPlanningCouncilShell returns the full council block: input
        // snapshot, participants (read-only availability + manual notes), and
        // the manual consensus / conflict / risk / assumption fields.
        function renderPlanningCouncilShell() {
            const c = councilState();
            const notes = c.providerNotes || [];
            const noteFor = (id) => (notes.find(n => n.providerId === id) || {}).note || '';
            const parts = (c.participants || []).length
                ? c.participants.map(p => participantCardHtml(p, noteFor(p.id))).join('')
                : intakeEmpty('참여자가 없습니다. "스냅샷 생성"으로 provider 가용성을 불러오세요.');
            const card = (typeof intakeCard === 'function') ? intakeCard : (t, b) => `<section class="intake-card"><h4>${t}</h4>${b}</section>`;
            const body = `<div class="council-block">
                <div class="q-toolbar"><button class="intake-btn active" onclick="generateCouncilSnapshot()">스냅샷 생성 / 참여자 새로고침</button><button class="intake-btn active" onclick="saveCouncil()">저장</button></div>
                <p class="up-note">🔒 실제 provider를 실행하지 않습니다. 각 AI 의견은 직접 붙여넣는 수동 입력입니다.</p>
                <h4 class="q-subtitle">입력 스냅샷</h4>${councilSnapshotSummary(c.inputSnapshot)}
                <h4 class="q-subtitle">참여자 (읽기 전용 가용성 + 수동 의견)</h4>${parts}
                <h4 class="q-subtitle">합의 (Consensus)</h4><textarea class="q-input" rows="2" placeholder="공통된 결론 초안" oninput="onCouncilConsensusInput(this)">${escC(c.consensus || '')}</textarea>
                ${councilListField('충돌 (Conflicts)', 'conflicts', '한 줄에 하나씩')}
                ${councilListField('리스크 (Risks)', 'risks', '한 줄에 하나씩')}
                ${councilListField('가정 (Assumptions)', 'assumptions', '한 줄에 하나씩')}
                ${councilListField('권고 (Recommendations)', 'recommendations', '한 줄에 하나씩')}
                ${councilListField('추가 질문 (Next Questions)', 'nextQuestions', '한 줄에 하나씩')}
            </div>`;
            // PR #26b: read-only provider prompt preview + manual import section.
            const prompts = (typeof renderProviderPrompts === 'function') ? renderProviderPrompts() : '';
            const imports = (typeof renderProviderImport === 'function') ? renderProviderImport() : '';
            const promptCard = (prompts || imports)
                ? card('Provider 프롬프트 / 응답 가져오기 (read-only)', prompts + imports)
                : '';
            // PR #30: client-only mock execution section. No provider run.
            const mock = (typeof renderProviderMockExecution === 'function') ? renderProviderMockExecution() : '';
            const mockCard = mock ? card('모의 실행 (Mock · read-only)', mock) : '';
            // PR #53: real provider run result summary (read-only, summary-only).
            const runResult = (typeof renderProviderRunResult === 'function') ? renderProviderRunResult() : '';
            const runResultCard = runResult ? card('실제 실행 결과 요약 (Real · summary-only)', runResult) : '';
            return `<h3 class="intake-section-title">Multi-AI Planning Council (수동)</h3>` + card('Planning Council', body, { badge: c.status }) + promptCard + mockCard + runResultCard;
        }
