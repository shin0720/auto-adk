        // PR #26a: Manual Planning Council templates + default builders.
        // NO provider run, NO /api/workflow/run, NO council execution backend.
        // Provider opinions are captured manually; availability is a read-only
        // snapshot from /api/providers/status. finalDecision is a draft that
        // stays approvedByUser=false until the user toggles it by hand.

        // Council participants mirror the read-only provider status list. The
        // per-participant status is one of the documented enum values.
        const COUNCIL_PARTICIPANTS = [
            { id: 'claude', name: 'Claude' },
            { id: 'codex',  name: 'Codex' },
            { id: 'gemini', name: 'Gemini' }
        ];

        // Documented participant status enum (manual mode never auto-runs, so
        // only available / unavailable / manualOnly are auto-derived; skipped
        // and failed are reserved for future provider-run PRs).
        const COUNCIL_STATUS_ENUM = ['available', 'unavailable', 'skipped', 'failed', 'manualOnly'];

        // Shared HTML escaper for council/decision fields.
        function escC(s) {
            return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
        }

        // buildCouncilInputSnapshot assembles the read-only planning context
        // from existing PR #23~#25 workflowState data. Client-side only.
        function buildCouncilInputSnapshot() {
            const s = workflowState;
            const intake = s.intake || {};
            const readiness = s.readiness || {};
            const uiux = s.uiuxDiscovery || {};
            const root = intake.originalRequest || (typeof currentRootRequest === 'function' ? currentRootRequest() : '');
            return {
                originalRequest: root,
                intakeAnswers: intake.answers || {},
                requiredAnswered: !!intake.requiredAnswered,
                userPrepChecklist: readiness.userPrepChecklist || [],
                designBrief: s.designBrief || {},
                questionRounds: uiux.questionRounds || [],
                referenceImages: uiux.referenceImages || [],
                referenceSites: uiux.referenceSites || [],
                nonGoals: intake.nonGoals || [],
                acceptanceCriteria: intake.acceptanceCriteria || [],
                costQuota: s.costQuota || {},
                rollback: s.rollback || {},
                maintenance: s.maintenance || {},
                capturedAt: new Date().toISOString()
            };
        }

        // defaultPlanningCouncil returns a fresh council object (new arrays each
        // call) so seeding never shares references across reloads.
        function defaultPlanningCouncil() {
            return {
                status: 'not_started',
                createdAt: '',
                inputSnapshot: null,
                participants: [],
                providerNotes: [],
                unavailableProviders: [],
                consensus: '',
                conflicts: [],
                risks: [],
                assumptions: [],
                recommendations: [],
                nextQuestions: []
            };
        }

        // defaultFinalDecision returns a fresh draft. approvedByUser and
        // readyForImplementation MUST default to false — no auto-approval.
        function defaultFinalDecision() {
            return {
                status: 'draft',
                selectedPlan: '',
                rationale: '',
                acceptedRisks: [],
                rejectedOptions: [],
                requiredUserApprovals: [],
                readyForImplementation: false,
                approvedByUser: false,
                approvedAt: ''
            };
        }

        // PR #26b: read-only provider prompt roles + contract. Prompts are for
        // the user to run externally and paste back — nothing is executed here.
        const PROVIDER_ROLES = {
            claude: '제품/기획 리스크',
            codex:  '기술 구조/구현 리스크',
            gemini: 'UIUX/사용자 플로우/대안'
        };

        // The contract is prepended to every provider prompt. It forbids any
        // mutation/execution and asks for design opinions only.
        const PROMPT_CONTRACT = [
            '아래는 읽기 전용 기획 검토 요청입니다. 반드시 다음 규칙을 지키세요:',
            '1) 파일을 수정하지 마세요. 2) 명령을 실행하지 마세요. 3) patch/diff를 만들지 마세요.',
            '4) 구현에 착수하지 마세요. 5) 설계 의견만 텍스트로 작성하세요.',
            '이 응답은 사용자의 최종 결정을 자동 승인하지 않으며 릴리스 준비와 무관합니다.'
        ].join('\n');

        // simpleHash is a small djb2 hash used to detect stale prompts when the
        // input snapshot changes. Not cryptographic.
        function simpleHash(str) {
            let h = 5381;
            for (let i = 0; i < str.length; i++) h = ((h << 5) + h + str.charCodeAt(i)) >>> 0;
            return h.toString(16);
        }

        // buildProviderPrompt assembles a role-specific read-only prompt from the
        // council input snapshot plus current manual notes. Client-side only.
        function buildProviderPrompt(providerId, snapshot, council) {
            const role = PROVIDER_ROLES[providerId] || '설계 리스크';
            const c = council || {};
            const ctx = {
                originalRequest: snapshot.originalRequest,
                intakeAnswers: snapshot.intakeAnswers,
                userPrepChecklist: snapshot.userPrepChecklist,
                questionRounds: snapshot.questionRounds,
                referenceImages: snapshot.referenceImages,
                referenceSites: snapshot.referenceSites,
                currentNotes: (c.providerNotes || []).map(n => ({ providerId: n.providerId, note: n.note })),
                currentConsensus: c.consensus || '',
                currentConflicts: c.conflicts || [],
                currentRisks: c.risks || []
            };
            return [
                PROMPT_CONTRACT,
                '',
                `당신의 검토 관점: ${role} (${providerId}).`,
                '',
                '## 입력 데이터 (JSON)',
                JSON.stringify(ctx, null, 2),
                '',
                '## 출력 형식 (아래 제목을 그대로 사용하세요)',
                '요약:', '위험:', '충돌:', '추천:', '질문:', '경고:',
                '각 항목은 한 줄에 하나씩, 없으면 "없음"으로 적으세요.'
            ].join('\n');
        }
