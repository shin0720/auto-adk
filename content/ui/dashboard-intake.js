        // PR #23: Planner intake / readiness / design-brief shells.
        // Data model + UI shell ONLY. No question generation, provider
        // execution, council, kickoff gate, or auto judgement lives here —
        // those arrive in later PRs. Buttons are disabled placeholders.

        // defaultIntakeStates returns fresh default objects (new arrays each
        // call) for every planning top-level key the shell renders.
        function defaultIntakeStates() {
            return {
                intake: { status: 'not_started', questions: [], answers: {}, requiredAnswered: false, assumptions: [], requirements: [], nonGoals: [], acceptanceCriteria: [] },
                readiness: { status: 'not_started', items: [], blockers: [], installPlan: [], userPrepChecklist: [], externalDependencies: [], realStubHumanTaskMap: [] },
                designBrief: { styleDirection: '', referenceProducts: [], designSystemRequirements: [], responsiveTargets: [1280, 768, 500], accessibilityRequirements: [], copyTone: '', productionPolishChecklist: [] },
                uiuxGate: { status: 'not_run', findings: [], screenshots: [], finalVerdict: '' },
                acceptanceEvidence: { evidenceRequired: false, evidenceItems: [], screenshots: [], testResults: [], smokeResults: [] },
                costQuota: { costRisks: [], quotaRisks: [], paidServiceRequired: false, userApprovalRequired: false },
                maintenance: { maintainer: '', handoffNotesRequired: false, userCanMaintain: false },
                rollback: { backupRequired: false, rollbackPlan: '', restorePoint: '' }
            };
        }

        // Seed defaults so the shell renders even before the first loadState().
        (function seedIntakeDefaults() {
            const d = defaultIntakeStates();
            for (const k in d) { if (!workflowState[k]) workflowState[k] = d[k]; }
        })();

        // Restore from server state; missing keys fall back to defaults so
        // legacy state.json files (without these keys) never break loadState.
        function restoreIntakeState(data) {
            const d = defaultIntakeStates();
            for (const k in d) { workflowState[k] = (data && data[k]) || d[k]; }
        }

        function intakeCard(title, bodyHtml, opts) {
            opts = opts || {};
            const badge = opts.badge ? `<span class="intake-badge b-gray">${opts.badge}</span>` : '';
            const btn = opts.btn ? `<button class="intake-btn" disabled title="다음 PR에서 활성화됩니다">${opts.btn}</button>` : '';
            return `<section class="intake-card"><header class="intake-card-head"><h4>${title}</h4><span class="intake-head-right">${badge}${btn}</span></header><div class="intake-card-body">${bodyHtml}</div></section>`;
        }

        function intakeEmpty(msg) {
            return `<p class="intake-empty">${msg}</p>`;
        }

        function renderIntakeShell() {
            const s = workflowState;
            const rows = [];
            rows.push('<h3 class="intake-section-title">기획 인테이크 &amp; 준비 점검</h3>');
            rows.push(intakeCard('기획 질문 (Intake Questions)', intakeEmpty('아직 질문이 없습니다 — 다음 단계에서 생성됩니다.'), { btn: '질문 생성', badge: s.intake.status }));
            rows.push(intakeCard('필수 / 선택 질문', intakeEmpty('질문이 생성되면 필수·선택으로 구분해 표시됩니다.')));
            rows.push(intakeCard('실행 준비 점검 (Readiness)', intakeEmpty('기술 스택·설치·의존성 분류가 여기에 표시됩니다.'), { btn: '분류 실행', badge: s.readiness.status }));
            rows.push(intakeCard('사용자 준비물 (User Preparation)', intakeEmpty('API 키·계정·도메인 등 사용자가 먼저 준비할 항목이 표시됩니다.'), { btn: '체크리스트 생성' }));
            rows.push(intakeCard('REAL / STUB / HUMAN_TASK / BLOCKED', '<div class="intake-legend"><span class="intake-tag t-real">REAL</span><span class="intake-tag t-stub">STUB</span><span class="intake-tag t-human">HUMAN_TASK</span><span class="intake-tag t-block">BLOCKED</span><span class="intake-tag t-defer">DEFERRED</span></div>' + intakeEmpty('외부 의존성 분류가 여기에 표시됩니다.')));
            rows.push(intakeCard('디자인 브리프 (Design Brief)', intakeEmpty('스타일 방향·참고 제품·톤이 확정되면 표시됩니다.'), { btn: 'AI 검토' }));
            rows.push(intakeCard('UI/UX 품질 게이트', '<div class="intake-verdict">판정: <span class="intake-badge b-gray">미실행</span></div>', { btn: '게이트 실행' }));
            rows.push(intakeCard('완료 증거 (Acceptance Evidence)', intakeEmpty('스크린샷·테스트·스모크 결과가 여기에 기록됩니다.')));
            rows.push(intakeCard('비용 / 할당량 (Cost / Quota)', intakeEmpty('유료 서비스·할당량 리스크가 여기에 표시됩니다.')));
            rows.push(intakeCard('유지보수 / 소유권 (Maintenance)', intakeEmpty('유지보수 담당·핸드오프 노트 필요 여부가 표시됩니다.')));
            rows.push(intakeCard('롤백 / 백업 (Rollback / Backup)', intakeEmpty('백업 필요 여부·롤백 계획·복원 지점이 표시됩니다.')));
            return rows.join('');
        }

        // openIntakePanel renders the intake + uiux-discovery shells into the
        // shared overlay panel. No agent is attached, so approval/forward
        // footers stay hidden.
        function openIntakePanel() {
            const content = document.getElementById('panel-content');
            content.style.padding = '';
            content.style.whiteSpace = 'normal';
            document.getElementById('panel-title').innerText = '기획 인테이크 · 준비 점검 · 디자인 브리프';
            const uiux = (typeof renderUiuxDiscoveryShell === 'function') ? renderUiuxDiscoveryShell() : '';
            content.innerHTML = `<div class="intake-shell">${renderIntakeShell()}${uiux}</div>`;
            document.getElementById('approval-footer').style.display = 'none';
            document.getElementById('forward-footer').style.display = 'none';
            document.getElementById('overlay-panel').style.display = 'flex';
        }
