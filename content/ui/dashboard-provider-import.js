        // PR #26b: manual import of pasted provider responses + failure-safe
        // parsing preview. rawText is ALWAYS preserved. NEVER touches
        // finalDecision (no auto-approval). Stored under planningCouncil.

        function importedResponsesList() {
            const c = councilState();
            if (!c.providerImportedResponses) c.providerImportedResponses = [];
            return c.providerImportedResponses;
        }

        function responseSummariesList() {
            const c = councilState();
            if (!c.providerResponseSummaries) c.providerResponseSummaries = [];
            return c.providerResponseSummaries;
        }

        function findOrCreate(list, providerId, make) {
            let e = list.find(x => x.providerId === providerId);
            if (!e) { e = make(); list.push(e); }
            return e;
        }

        // onProviderRawInput stores the pasted text as the user types. No parse
        // and no saveState on each keystroke; import commits it.
        function onProviderRawInput(providerId, value) {
            const e = findOrCreate(importedResponsesList(), providerId,
                () => ({ providerId, rawText: '', parsed: false, parsedAt: '', parseStatus: 'pasted', source: 'manualPaste' }));
            e.rawText = value;
        }

        function setPromptStatus(providerId, status) {
            const p = (councilState().providerPrompts || []).find(x => x.providerId === providerId);
            if (p) p.status = status;
        }

        // importProviderResponse commits the pasted rawText and runs the local
        // failure-safe parser. On parse failure the rawText is kept and the
        // status becomes failedParse; finalDecision is never modified.
        function importProviderResponse(providerId) {
            const resp = findOrCreate(importedResponsesList(), providerId,
                () => ({ providerId, rawText: '', parsed: false, parsedAt: '', parseStatus: 'pasted', source: 'manualPaste' }));
            const raw = (resp.rawText || '').trim();
            if (!raw) { intakeToast('붙여넣은 응답이 없습니다.'); return; }
            const result = parseProviderResponse(raw);
            resp.parsed = result.parseStatus === 'parsed';
            resp.parseStatus = result.parseStatus;
            resp.parsedAt = new Date().toISOString();
            resp.source = 'manualPaste';
            const sum = findOrCreate(responseSummariesList(), providerId, () => ({ providerId }));
            sum.summary = result.summary;
            sum.risks = result.risks;
            sum.conflicts = result.conflicts;
            sum.recommendations = result.recommendations;
            sum.missingQuestions = result.missingQuestions;
            sum.implementationWarnings = result.implementationWarnings;
            setPromptStatus(providerId, resp.parsed ? 'parsed' : 'failedParse');
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(resp.parsed ? `${providerId} 응답을 파싱했습니다.` : `${providerId} 파싱 실패 — 원문은 저장되었습니다.`);
        }

        function importBadge(status) {
            const map = { pasted: 'b-amber', parsed: 'b-green', failedParse: 'b-red' };
            return `<span class="intake-badge ${map[status] || 'b-gray'}">${status || 'notImported'}</span>`;
        }

        function summaryPreviewHtml(sum) {
            if (!sum) return '';
            const listBlock = (label, arr) => (arr && arr.length)
                ? `<div class="pi-field"><span class="pi-label">${label}</span><ul>${arr.map(x => `<li>${escC(x)}</li>`).join('')}</ul></div>` : '';
            const summary = sum.summary ? `<div class="pi-field"><span class="pi-label">요약</span><p>${escC(sum.summary)}</p></div>` : '';
            return `<div class="pi-summary">${summary}${listBlock('위험', sum.risks)}${listBlock('충돌', sum.conflicts)}${listBlock('추천', sum.recommendations)}${listBlock('질문', sum.missingQuestions)}${listBlock('경고', sum.implementationWarnings)}</div>`;
        }

        function providerImportCardHtml(providerId, name) {
            const resp = importedResponsesList().find(x => x.providerId === providerId) || {};
            const sum = responseSummariesList().find(x => x.providerId === providerId);
            const failedNote = resp.parseStatus === 'failedParse'
                ? `<p class="council-reason">파싱에 실패했지만 원문은 저장되었습니다. 아래 원문을 그대로 활용하세요.</p>` : '';
            const preview = resp.parseStatus === 'parsed' ? summaryPreviewHtml(sum) : '';
            return `<div class="pi-card"><div class="up-row"><strong>${escC(name)}</strong>${importBadge(resp.parseStatus)}</div>`
                + `<textarea class="q-input" rows="4" placeholder="외부 AI 응답을 여기에 붙여넣으세요 (원문은 항상 보존됩니다)" oninput="onProviderRawInput('${providerId}', this.value)">${escC(resp.rawText || '')}</textarea>`
                + `<div class="q-toolbar"><button class="intake-btn active" onclick="importProviderResponse('${providerId}')">응답 가져오기 (파싱)</button></div>`
                + failedNote + preview + `</div>`;
        }

        // renderProviderImport returns the paste/import block for all providers.
        function renderProviderImport() {
            const cards = COUNCIL_PARTICIPANTS.map(p => providerImportCardHtml(p.id, p.name)).join('');
            return `<div class="pi-block">
                <h4 class="q-subtitle">응답 붙여넣기 / 가져오기 (수동)</h4>
                <p class="up-note">파싱은 best-effort이며 실패해도 원문(rawText)은 보존됩니다. finalDecision은 자동 변경되지 않습니다.</p>
                ${cards}
            </div>`;
        }
