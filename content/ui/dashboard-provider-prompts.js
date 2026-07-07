        // PR #26b: read-only provider prompt generation + copy. Builds a prompt
        // per provider for the user to run EXTERNALLY. NO provider run, NO
        // /api/workflow/run, NO auth. Stored under planningCouncil.providerPrompts.

        function providerPromptsList() {
            const c = councilState();
            if (!c.providerPrompts) c.providerPrompts = [];
            return c.providerPrompts;
        }

        // generateProviderPrompts (re)builds a read-only prompt for every
        // participant from the current input snapshot. Existing prompt status is
        // reset to promptReady; nothing is executed.
        function generateProviderPrompts() {
            const c = councilState();
            const snap = buildCouncilInputSnapshot();
            const list = COUNCIL_PARTICIPANTS.map(p => {
                const prompt = buildProviderPrompt(p.id, snap, c);
                return {
                    providerId: p.id,
                    role: PROVIDER_ROLES[p.id] || '',
                    prompt,
                    createdAt: new Date().toISOString(),
                    inputHash: simpleHash(prompt),
                    status: 'promptReady'
                };
            });
            c.providerPrompts = list;
            saveState();
            renderIntakePanelIfOpen();
            intakeToast('Provider 프롬프트를 생성했습니다. 복사해 외부에서 실행하세요.');
        }

        // copyProviderPrompt copies to the clipboard. On failure the prompt text
        // stays visible so the user can select it manually.
        function copyProviderPrompt(providerId) {
            const item = providerPromptsList().find(p => p.providerId === providerId);
            if (!item) return;
            const done = () => intakeToast(`${providerId} 프롬프트를 복사했습니다.`);
            const fail = () => intakeToast('복사 실패 — 아래 프롬프트를 직접 선택해 복사하세요.');
            try {
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(item.prompt).then(done).catch(fail);
                } else { fail(); }
            } catch (_) { fail(); }
        }

        function ppBadge(status) {
            const map = { notGenerated: 'b-gray', promptReady: 'b-amber', pasted: 'b-amber', parsed: 'b-green', failedParse: 'b-red' };
            return `<span class="intake-badge ${map[status] || 'b-gray'}">${status}</span>`;
        }

        function providerPromptCardHtml(p, participant) {
            // Support reviewers (e.g. Gemini) are optional; their absence never
            // blocks the Council. Primary reviewers (Claude/Codex) carry no note.
            const isSupport = typeof PROVIDER_PRIMARY === 'object' && PROVIDER_PRIMARY[p.providerId] === false;
            const supportTag = isSupport ? `<span class="pp-support">선택 보조</span>` : '';
            const supportNote = isSupport
                ? `<p class="council-reason">${escC((typeof PROVIDER_TIER_NOTE === 'object' && PROVIDER_TIER_NOTE[p.providerId]) || '선택 보조 검토자입니다. 응답이 없어도 진행할 수 있습니다.')}</p>`
                : '';
            const unavail = participant && participant.status === 'unavailable'
                ? `<p class="council-reason">이 provider는 로컬에서 unavailable입니다. 프롬프트만 제공되며 수동 실행/붙여넣기만 가능합니다 (manualOnly)${isSupport ? ' — 선택 보조라 진행에는 영향이 없습니다' : ''}.</p>`
                : '';
            return `<div class="pp-card"><div class="up-row"><strong>${escC(participant ? participant.name : p.providerId)}</strong><span class="pp-role">${escC(p.role)}</span>${supportTag}${ppBadge(p.status)}</div>`
                + supportNote + unavail
                + `<details class="pp-details"><summary>프롬프트 보기 / 복사</summary>`
                + `<div class="q-toolbar"><button class="intake-btn active" onclick="copyProviderPrompt('${p.providerId}')">복사</button></div>`
                + `<textarea class="q-input pp-prompt" rows="6" readonly>${escC(p.prompt)}</textarea></details></div>`;
        }

        // renderProviderPrompts returns the prompt-generation block. Called from
        // the planning-council shell; unavailable providers still get a prompt.
        function renderProviderPrompts() {
            const c = councilState();
            const prompts = c.providerPrompts || [];
            const partById = {};
            (c.participants || []).forEach(p => { partById[p.id] = p; });
            const cards = prompts.length
                ? prompts.map(p => providerPromptCardHtml(p, partById[p.providerId] || { name: p.providerId })).join('')
                : intakeEmpty('아직 프롬프트가 없습니다. "Provider 프롬프트 생성"을 눌러 read-only 검토 프롬프트를 만드세요.');
            return `<div class="pp-block">
                <div class="q-toolbar"><button class="intake-btn active" onclick="generateProviderPrompts()">Provider 프롬프트 생성</button></div>
                <p class="up-note">🔒 provider를 실행하지 않습니다. 프롬프트를 복사해 외부 AI/터미널/웹에서 실행한 뒤 결과를 아래에 붙여넣으세요.</p>
                ${cards}
            </div>`;
        }
