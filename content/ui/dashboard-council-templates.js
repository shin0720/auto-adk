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
