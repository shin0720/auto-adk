        // PR #30: client-only MOCK provider execution. There is NO provider run,
        // NO /api/workflow/run, NO backend endpoint, NO auth and NO LLM call. It
        // only simulates the running -> completed/failed/canceled UI/state flow so
        // real execution can later be wired behind a safety gate. rawText is ALWAYS
        // preserved, canAutoRun stays false, and finalDecision is NEVER modified.
        // Stored under planningCouncil.providerMockResponses (RawMessage passthrough;
        // ui_state.go is not touched). The parsed preview is kept on the mock entry
        // itself (parsedFields) rather than the shared providerResponseSummaries[],
        // so mock data can never contaminate the PR #27 manual-import preview.

        // In-memory setTimeout handles per provider. Not persisted; a reload simply
        // shows the last saved status (a mid-run reload never resumes a fake timer).
        const mockTimers = {};

        function mockResponsesList() {
            const c = councilState();
            if (!c.providerMockResponses) c.providerMockResponses = [];
            return c.providerMockResponses;
        }

        function mockEntry(providerId) {
            const list = mockResponsesList();
            let e = list.find(x => x.providerId === providerId);
            if (!e) {
                e = { providerId, source: 'mock', status: 'idle', rawText: '', parsed: false,
                    parseStatus: '', startedAt: '', finishedAt: '', error: '', inputHash: '', scenario: '', parsedFields: null };
                list.push(e);
            }
            return e;
        }

        // Canned sample opinions. Rendered as a labelled header line + bare JSON so
        // the existing parser's JSON path recovers them. Every sample is prefixed
        // with "[모의 샘플]" so it can never be mistaken for a real response.
        const MOCK_CANNED = {
            claude: { summary: '[모의 샘플] 제품/기획 관점 검토 결과입니다. 실제 실행이 아닙니다.',
                risks: ['무료/유료 경계가 미정', '타겟 사용자 정의가 좁을 수 있음'],
                recommendations: ['MVP 범위를 핵심 1개 기능으로 축소', '온보딩 흐름 단순화'],
                missingQuestions: ['결제 정책이 확정됐나요?', '성공 지표(KPI)는 무엇인가요?'],
                implementationWarnings: ['상태 저장 스키마 변경 시 마이그레이션 필요'] },
            codex: { summary: '[모의 샘플] 기술 구조/구현 리스크 검토 결과입니다. 실제 실행이 아닙니다.',
                risks: ['단일 상태 파일 동시성 위험', '클라이언트 전용 저장의 무결성 한계'],
                recommendations: ['state round-trip 테스트 추가', '버전 필드 기반 낙관적 잠금 검토'],
                implementationWarnings: ['300줄 파일 한도 준수', 'backend endpoint 추가 지양'] },
            gemini: { summary: '[모의 샘플] 선택 보조 관점(UIUX/흐름/대안) 참고 의견입니다. 없어도 진행 가능합니다.',
                conflicts: ['버튼이 많아 좁은 화면에서 혼잡할 수 있음'],
                recommendations: ['모의/실제 구분을 배지로 명확히', '빈 상태 문구 보강'],
                missingQuestions: ['대안 흐름(B안)을 검토했나요?'] }
        };

        function cannedRaw(providerId) {
            const obj = MOCK_CANNED[providerId] || MOCK_CANNED.claude;
            return '[모의 샘플] 실제 provider 응답이 아닙니다. 아래는 파싱 검증용 JSON입니다.\n' + JSON.stringify(obj, null, 2);
        }

        // Deliberately unstructured (no headers, no JSON) so parseProviderResponse
        // returns failedParse while the rawText is still preserved.
        const MOCK_FAILPARSE = '[모의 샘플] 의도적으로 구조화되지 않은 원문입니다 파서가 섹션을 찾지 못해 failedParse가 되어야 하며 원문은 그대로 보존되어야 합니다';

        // runMock starts a fake run. scenario: success | error | failParse | timeout.
        // A short setTimeout stands in for execution; nothing is actually run.
        function runMock(providerId, scenario) {
            const e = mockEntry(providerId);
            if (e.status === 'mockRunning') return;
            e.source = 'mock';
            e.scenario = scenario;
            e.status = 'mockRunning';
            e.startedAt = new Date().toISOString();
            e.finishedAt = '';
            e.error = '';
            e.parsed = false;
            e.parseStatus = '';
            e.parsedFields = null;
            e.rawText = '';
            e.inputHash = '';
            renderIntakePanelIfOpen();
            const delay = scenario === 'timeout' ? 900 : 650;
            mockTimers[providerId] = setTimeout(() => finishMock(providerId, scenario), delay);
            intakeToast(`${providerId} 모의 실행 중… (실제 AI는 실행되지 않습니다)`);
        }

        // finishMock resolves a fake run. It never throws and never touches
        // finalDecision. rawText is preserved on every branch that produced one.
        function finishMock(providerId, scenario) {
            delete mockTimers[providerId];
            const e = mockEntry(providerId);
            if (e.status !== 'mockRunning') return; // canceled before firing
            e.finishedAt = new Date().toISOString();
            if (scenario === 'error' || scenario === 'timeout') {
                e.status = 'mockFailed';
                e.error = scenario === 'timeout' ? '모의 타임아웃' : '모의 실행 오류(샘플): provider가 응답을 반환하지 않았습니다.';
                e.rawText = '';
                saveState();
                renderIntakePanelIfOpen();
                intakeToast(`${providerId} ${e.error}`);
                return;
            }
            const raw = scenario === 'failParse' ? MOCK_FAILPARSE : cannedRaw(providerId);
            e.rawText = raw;
            e.inputHash = (typeof simpleHash === 'function') ? simpleHash(raw) : '';
            const result = parseProviderResponse(raw);
            e.parsed = result.parseStatus === 'parsed';
            e.parseStatus = result.parseStatus;
            e.parsedFields = e.parsed ? {
                summary: result.summary, risks: result.risks, conflicts: result.conflicts,
                recommendations: result.recommendations, missingQuestions: result.missingQuestions,
                implementationWarnings: result.implementationWarnings
            } : null;
            e.status = 'mockCompleted';
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(e.parsed ? `${providerId} 모의 완료 — 파싱 성공.` : `${providerId} 모의 완료 — 파싱 실패, 원문은 보존됨.`);
        }

        // cancelMock stops an in-flight fake run. Any partial rawText is preserved.
        function cancelMock(providerId) {
            const t = mockTimers[providerId];
            if (t) { clearTimeout(t); delete mockTimers[providerId]; }
            const e = mockEntry(providerId);
            if (e.status !== 'mockRunning') return;
            e.status = 'mockCanceled';
            e.finishedAt = new Date().toISOString();
            e.error = '사용자가 모의 실행을 취소했습니다.';
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(`${providerId} 모의 실행을 취소했습니다.`);
        }

        function mockBadge(status) {
            const cls = { idle: 'b-gray', mockRunning: 'b-amber', mockCompleted: 'b-green', mockFailed: 'b-red', mockCanceled: 'b-gray' };
            const label = { idle: '모의 대기', mockRunning: '모의 실행 중', mockCompleted: '모의 완료', mockFailed: '모의 실패', mockCanceled: '모의 취소' };
            return `<span class="intake-badge ${cls[status] || 'b-gray'}">${label[status] || '모의 대기'}</span>`;
        }

        function mockCardHtml(participant) {
            const id = participant.id, name = participant.name || id;
            const e = mockResponsesList().find(x => x.providerId === id) || { status: 'idle' };
            const isSupport = typeof PROVIDER_PRIMARY === 'object' && PROVIDER_PRIMARY[id] === false;
            const roleTag = isSupport ? '선택 보조' : '메인 검토';
            const running = e.status === 'mockRunning';
            const errNote = (e.status === 'mockFailed' || e.status === 'mockCanceled') && e.error
                ? `<p class="council-reason">사유: ${escC(e.error)}</p>` : '';
            const failedNote = e.parseStatus === 'failedParse'
                ? `<p class="council-reason">파싱에 실패했지만 원문(rawText)은 저장되었습니다.</p>` : '';
            const rawBlock = e.rawText
                ? `<details class="pp-details"><summary>원문 보기 (모의 샘플)</summary><textarea class="q-input pp-prompt" rows="6" readonly>${escC(e.rawText)}</textarea></details>` : '';
            const preview = (e.status === 'mockCompleted' && e.parsed && e.parsedFields && typeof summaryPreviewHtml === 'function')
                ? summaryPreviewHtml(e.parsedFields) : '';
            const btns = running
                ? `<button class="intake-btn active" onclick="cancelMock('${id}')">취소</button>`
                : `<button class="intake-btn active" onclick="runMock('${id}','success')">모의 실행</button>`
                    + `<button class="intake-btn" onclick="runMock('${id}','error')">모의 실패 테스트</button>`
                    + `<button class="intake-btn" onclick="runMock('${id}','failParse')">모의 파싱 실패</button>`
                    + `<button class="intake-btn" onclick="runMock('${id}','timeout')">모의 타임아웃</button>`;
            return `<div class="mock-card"><div class="up-row"><strong>${escC(name)}</strong><span class="pp-role">${escC(roleTag)}</span>`
                + `${isSupport ? '<span class="pp-support">선택 보조</span>' : ''}${mockBadge(e.status || 'idle')}</div>`
                + `<div class="q-toolbar mock-toolbar">${btns}</div>`
                + errNote + failedNote + rawBlock + preview
                + `<p class="mock-fallback">응답이 필요하면 위 “응답 붙여넣기 / 가져오기” 섹션에서 수동으로 대체할 수 있습니다 (manual import).</p></div>`;
        }

        // renderProviderMockExecution returns the mock section. Participants come
        // from the council snapshot when present, else the static participant list.
        function renderProviderMockExecution() {
            const c = councilState();
            const parts = (c.participants && c.participants.length)
                ? c.participants
                : COUNCIL_PARTICIPANTS.map(p => ({ id: p.id, name: p.name }));
            const cards = parts.map(p => mockCardHtml(p)).join('');
            return `<div class="mock-block">
                <h4 class="q-subtitle">모의 실행 (Mock)</h4>
                <p class="up-note">🔒 실제 AI를 실행하지 않습니다. 할당량을 사용하지 않는 모의 흐름입니다. 실제 실행은 후속 안전 PR 이후에만 검토합니다.</p>
                ${cards}
            </div>`;
        }
