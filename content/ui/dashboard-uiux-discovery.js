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
            rows.push(card('디자인 질문 라운드', empty('1차 취향 · 2차 레퍼런스 · 3차 재질문 라운드가 표시됩니다.'), { btn: '질문 시작', badge: d.status }));
            rows.push(card('참고 이미지 (Reference Images)', empty('업로드된 참고 이미지 메타데이터가 표시됩니다. 원본을 복제하지 않고 시각 원칙만 추출합니다.'), { btn: '이미지 첨부' }));
            rows.push(card('참고 사이트 (Reference Sites)', empty('참고 서비스·URL이 표시됩니다.')));
            rows.push(card('추출된 시각 원칙 (Visual Principles)', empty('색·타이포·레이아웃·여백 원칙이 추출되면 표시됩니다.')));
            rows.push(card('디자인 방향 후보 (Visual Directions)', empty('2~4개 방향 후보가 제안되면 표시됩니다. 특정 브랜드 복제가 아닌 영감 방향으로만 기록됩니다.'), { btn: '방향 제안' }));
            rows.push(card('선택한 방향 (Selected Direction)', empty('아직 선택된 방향이 없습니다.')));
            rows.push(card('싫어하는 스타일 (Rejected Styles)', empty('피해야 할 스타일이 여기에 기록됩니다.')));
            rows.push(card('디자인 승인 상태', '<div class="intake-verdict">승인: <span class="intake-badge b-gray">' + d.designApprovalStatus + '</span></div>'));
            rows.push(card('가정 (Assumptions)', empty('AI가 임의로 채운 가정은 [AI-GUESSED] 태그로 표시됩니다.')));
            return rows.join('');
        }
