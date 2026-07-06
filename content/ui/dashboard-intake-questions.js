        // PR #25: template-based intake question generation + answer capture.
        // Deterministic, no provider call. Questions/answers persist through the
        // existing saveState/loadState path (workflowState.intake.*).

        // generateIntakeQuestions builds the intake+readiness question set sized by
        // the clarity tier of the seed request. Required questions are always
        // included; optional ones fill up to the tier maximum. Existing answers
        // are preserved across regeneration.
        function generateIntakeQuestions() {
            const seed = (workflowState.intake && workflowState.intake.requirements && workflowState.intake.originalRequest) || '';
            const rootSeed = seed || currentRootRequest();
            const tier = clarityTier(rootSeed);
            const pool = INTAKE_QUESTION_TEMPLATES.concat(READINESS_QUESTION_TEMPLATES);
            const required = pool.filter(q => q.required);
            const optional = pool.filter(q => !q.required);
            const room = Math.max(0, tier.max - required.length);
            const chosen = required.concat(optional.slice(0, room));
            workflowState.intake.questions = chosen.map(q => ({ ...q }));
            if (!workflowState.intake.answers) workflowState.intake.answers = {};
            workflowState.intake.status = 'questioning';
            recomputeRequiredAnswered();
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(`기획 질문 ${chosen.length}개 생성 (${tier.tier})`);
        }

        function currentRootRequest() {
            try {
                const firstActive = (workflowState.nodes || []).find(n => n.originalRequest);
                return firstActive ? firstActive.originalRequest : '';
            } catch (_) { return ''; }
        }

        function recomputeRequiredAnswered() {
            const qs = (workflowState.intake && workflowState.intake.questions) || [];
            const req = qs.filter(q => q.required);
            const ans = workflowState.intake.answers || {};
            const done = req.filter(q => (ans[q.id] || '').trim().length > 0).length;
            workflowState.intake.requiredAnswered = req.length > 0 && done === req.length;
            return { done, total: req.length };
        }

        // onAnswerInput mirrors textarea/input edits into workflowState (no save yet).
        function onAnswerInput(el) {
            const qid = el.dataset.qid;
            if (!qid) return;
            if (!workflowState.intake.answers) workflowState.intake.answers = {};
            workflowState.intake.answers[qid] = el.value;
            const p = recomputeRequiredAnswered();
            const bar = document.getElementById('intake-progress-bar');
            const lbl = document.getElementById('intake-progress-label');
            if (bar) bar.style.width = (p.total ? Math.round(p.done / p.total * 100) : 0) + '%';
            if (lbl) lbl.textContent = `필수 답변 ${p.done}/${p.total}`;
            const card = el.closest('.q-card');
            if (card && card.dataset.required === 'true') {
                card.classList.toggle('q-missing', (el.value || '').trim().length === 0);
            }
        }

        function saveIntakeAnswers() {
            recomputeRequiredAnswered();
            saveState();
            intakeToast('답변이 저장되었습니다.');
        }

        function questionCardHtml(q) {
            const ans = (workflowState.intake.answers || {})[q.id] || '';
            const missing = q.required && ans.trim().length === 0;
            const reqBadge = q.required ? '<span class="q-req">필수</span>' : '<span class="q-opt">선택</span>';
            const esc = s => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
            const field = q.type === 'text'
                ? `<input class="q-input" type="text" data-qid="${q.id}" value="${esc(ans)}" oninput="onAnswerInput(this)" placeholder="답변을 입력하세요">`
                : `<textarea class="q-input" data-qid="${q.id}" rows="2" oninput="onAnswerInput(this)" placeholder="답변을 입력하세요">${esc(ans)}</textarea>`;
            return `<div class="q-card${missing ? ' q-missing' : ''}" data-required="${q.required}"><div class="q-head"><span class="q-cat">${q.category}</span>${reqBadge}</div><div class="q-text">${esc(q.text)}</div>${field}</div>`;
        }

        // renderIntakeQuestions returns the question/answer block for the intake
        // panel: progress bar + required cards + a collapsed optional section.
        function renderIntakeQuestions() {
            const qs = (workflowState.intake && workflowState.intake.questions) || [];
            if (qs.length === 0) {
                return `<div class="q-block"><button class="intake-btn active" onclick="generateIntakeQuestions()">기획 질문 생성</button>${intakeEmpty('아직 질문이 없습니다. "기획 질문 생성"을 눌러 시작하세요.')}</div>`;
            }
            const p = recomputeRequiredAnswered();
            const pct = p.total ? Math.round(p.done / p.total * 100) : 0;
            const req = qs.filter(q => q.required).map(questionCardHtml).join('');
            const opt = qs.filter(q => !q.required).map(questionCardHtml).join('');
            const optBlock = opt ? `<details class="q-optional"><summary>선택 질문 ${qs.filter(q => !q.required).length}개 펼치기</summary>${opt}</details>` : '';
            return `<div class="q-block">
                <div class="q-toolbar"><button class="intake-btn active" onclick="generateIntakeQuestions()">질문 다시 생성</button><button class="intake-btn active" onclick="saveIntakeAnswers()">답변 저장</button></div>
                <div class="q-progress"><div class="q-progress-track"><div id="intake-progress-bar" class="q-progress-fill" style="width:${pct}%"></div></div><span id="intake-progress-label" class="q-progress-label">필수 답변 ${p.done}/${p.total}</span></div>
                ${req}${optBlock}
            </div>`;
        }

        // intakeToast shows a transient save/generate confirmation.
        function intakeToast(msg) {
            let el = document.getElementById('intake-toast');
            if (!el) {
                el = document.createElement('div');
                el.id = 'intake-toast';
                el.className = 'intake-toast';
                document.body.appendChild(el);
            }
            el.textContent = msg;
            el.classList.add('show');
            clearTimeout(el._t);
            el._t = setTimeout(() => el.classList.remove('show'), 1800);
        }

        // renderIntakePanelIfOpen re-renders the shell when the intake panel is visible.
        function renderIntakePanelIfOpen() {
            const panel = document.getElementById('overlay-panel');
            if (panel && panel.style.display === 'flex' && typeof openIntakePanel === 'function') {
                openIntakePanel();
            }
        }
