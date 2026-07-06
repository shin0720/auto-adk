        // PR #23: UI/UX design discovery shell. Metadata display ONLY —
        // no image upload, no image analysis, no direction generation, no
        // auto judgement. Reference handling extracts principles (never
        // clones); those flows arrive in later PRs. Buttons are disabled.

        function defaultUiuxDiscovery() {
            return {
                status: 'not_started',
                questionRounds: [],
                answers: {},
                referenceImages: [],
                referenceSites: [],
                extractedVisualPrinciples: [],
                visualDirectionOptions: [],
                selectedDirection: null,
                rejectedStyles: [],
                designApprovalStatus: 'pending',
                approvedAt: '',
                assumptions: []
            };
        }

        // Seed default so the shell renders before the first loadState().
        (function seedUiuxDiscoveryDefault() {
            if (!workflowState.uiuxDiscovery) workflowState.uiuxDiscovery = defaultUiuxDiscovery();
        })();

        // Restore from server state; a missing key falls back to the default
        // so legacy state.json files load without error.
        function restoreUiuxDiscoveryState(data) {
            workflowState.uiuxDiscovery = (data && data.uiuxDiscovery) || defaultUiuxDiscovery();
        }

        // renderUiuxDiscoveryShell reuses intakeCard/intakeEmpty from
        // dashboard-intake.js (loaded first) to keep styling consistent.
        function renderUiuxDiscoveryShell() {
            const d = workflowState.uiuxDiscovery || defaultUiuxDiscovery();
            const card = (typeof intakeCard === 'function') ? intakeCard : (t, b) => `<section class="intake-card"><h4>${t}</h4>${b}</section>`;
            const empty = (typeof intakeEmpty === 'function') ? intakeEmpty : (m) => `<p class="intake-empty">${m}</p>`;
            const rows = [];
            rows.push('<h3 class="intake-section-title">UI/UX 디자인 디스커버리</h3>');
            const roundsBody = (typeof renderDesignRounds === 'function') ? renderDesignRounds() : empty('디자인 질문 모듈 로드 대기 중…');
            rows.push(card('디자인 질문 · 레퍼런스 (Design Rounds)', roundsBody, { badge: d.status }));
            rows.push(card('디자인 방향 후보 (Visual Directions)', empty('2~4개 방향 후보 제안은 다음 PR(Council)에서 자동 생성됩니다. 현재는 수동 선택만 지원됩니다.')));
            rows.push(card('디자인 승인 상태', '<div class="intake-verdict">승인: <span class="intake-badge b-gray">' + d.designApprovalStatus + '</span></div>'));
            rows.push(card('가정 (Assumptions)', empty('AI가 임의로 채운 가정은 [AI-GUESSED] 태그로 표시됩니다.')));
            return rows.join('');
        }
