        // PR #26a: finalDecision draft + manual approval gate. approvedByUser
        // and readyForImplementation default to false and only change via the
        // manual toggle below. Approval NEVER triggers a worker kickoff and is
        // unrelated to release readiness — both are stated in the UI.

        // Seed default so the shell renders before the first loadState().
        (function seedFinalDecisionDefault() {
            if (!workflowState.finalDecision) workflowState.finalDecision = defaultFinalDecision();
        })();

        // Restore from server state; a missing key falls back to the default so
        // legacy state.json files load without error.
        function restoreFinalDecisionState(data) {
            workflowState.finalDecision = (data && data.finalDecision) || defaultFinalDecision();
        }

        function fdState() {
            if (!workflowState.finalDecision) workflowState.finalDecision = defaultFinalDecision();
            return workflowState.finalDecision;
        }

        function onFdTextInput(field, el) {
            fdState()[field] = el.value;
        }

        function onFdListInput(field, el) {
            fdState()[field] = el.value.split('\n').map(s => s.trim()).filter(Boolean);
        }

        function saveFinalDecision() {
            saveState();
            intakeToast('finalDecision 초안이 저장되었습니다.');
        }

        // toggleFinalApproval is the ONLY path that sets approvedByUser=true.
        // It is a manual user action; nothing auto-approves. readyForImplementation
        // mirrors the approval flag as a data marker but triggers no kickoff.
        function toggleFinalApproval() {
            const d = fdState();
            d.approvedByUser = !d.approvedByUser;
            if (d.approvedByUser) {
                d.approvedAt = new Date().toISOString();
                d.status = 'approved';
                d.readyForImplementation = true;
            } else {
                d.approvedAt = '';
                d.status = 'draft';
                d.readyForImplementation = false;
            }
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(d.approvedByUser ? '사용자 승인됨 (구현 자동 실행 없음).' : '승인이 해제되었습니다.');
        }

        function fdListField(label, field, ph) {
            const val = (fdState()[field] || []).join('\n');
            return `<h4 class="q-subtitle">${label}</h4><textarea class="q-input" rows="2" placeholder="${ph}" oninput="onFdListInput('${field}', this)">${escC(val)}</textarea>`;
        }

        function fdUnavailableNote() {
            const c = workflowState.planningCouncil || {};
            const un = c.unavailableProviders || [];
            if (un.length === 0) return '';
            const names = un.map(u => escC(u.id)).join(', ');
            return `<p class="council-reason">⚠️ unavailable provider(${names})가 있습니다. 결정 근거에 반영하세요.</p>`;
        }

        // renderFinalDecision returns the decision draft card. The approval
        // banner and release-ready disclaimer are always visible.
        function renderFinalDecision() {
            const d = fdState();
            const card = (typeof intakeCard === 'function') ? intakeCard : (t, b) => `<section class="intake-card"><h4>${t}</h4>${b}</section>`;
            const approved = d.approvedByUser;
            const banner = approved
                ? `<div class="fd-banner fd-ok">✅ 사용자 승인됨 · 승인 시각 ${escC(d.approvedAt)}<br>이 승인은 구현(worker kickoff)을 자동 실행하지 않으며, 릴리스 준비(release ready)와 무관합니다.</div>`
                : `<div class="fd-banner fd-warn">⚠️ 이 초안은 승인 전입니다. 승인 전에는 구현(kickoff)을 진행하지 마세요. 승인하더라도 kickoff는 자동 실행되지 않고, 릴리스 준비와도 무관합니다.</div>`;
            const body = `<div class="fd-block">
                ${banner}${fdUnavailableNote()}
                <div class="q-toolbar"><button class="intake-btn active" onclick="saveFinalDecision()">저장</button><button class="intake-btn ${approved ? 'active' : ''}" onclick="toggleFinalApproval()">${approved ? '승인 해제' : '사용자 승인'}</button></div>
                <h4 class="q-subtitle">선택한 계획 (Selected Plan)</h4><textarea class="q-input" rows="2" placeholder="채택한 계획 초안" oninput="onFdTextInput('selectedPlan', this)">${escC(d.selectedPlan || '')}</textarea>
                <h4 class="q-subtitle">근거 (Rationale)</h4><textarea class="q-input" rows="3" placeholder="왜 이 계획인지 · 리스크/충돌/가정 근거 포함" oninput="onFdTextInput('rationale', this)">${escC(d.rationale || '')}</textarea>
                ${fdListField('수용한 리스크 (Accepted Risks)', 'acceptedRisks', '한 줄에 하나씩')}
                ${fdListField('기각한 옵션 (Rejected Options)', 'rejectedOptions', '한 줄에 하나씩')}
                ${fdListField('필요한 사용자 승인 항목 (Required User Approvals)', 'requiredUserApprovals', '한 줄에 하나씩')}
                <p class="up-note">approvedByUser=${approved} · readyForImplementation=${d.readyForImplementation} — 두 값은 수동 승인으로만 바뀌며 kickoff를 실행하지 않습니다.</p>
            </div>`;
            return `<h3 class="intake-section-title">최종 결정 (finalDecision · 초안)</h3>` + card('finalDecision', body, { badge: d.status });
        }
