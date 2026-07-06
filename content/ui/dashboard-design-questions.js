        // PR #25: UIUX design question rounds + reference metadata capture.
        // Metadata ONLY — no image upload, no image analysis, no auto direction
        // generation, no clone-risk judgement. Persists via
        // workflowState.uiuxDiscovery (answers/questionRounds/references).

        // generateDesignQuestions creates round-1 taste questions. Round-2/3
        // shells appear conditionally (reference-based / rejected re-ask).
        function generateDesignQuestions() {
            const d = workflowState.uiuxDiscovery;
            const round1 = UIUX_QUESTION_TEMPLATES.filter(q => q.round === 1).map(q => ({ ...q }));
            d.questionRounds = [{ round: 1, questions: round1, generatedAt: new Date().toISOString() }];
            d.status = 'questioning';
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(`디자인 질문 ${round1.length}개 생성 (1차)`);
        }

        function onDesignAnswerInput(el) {
            const qid = el.dataset.qid;
            if (!qid) return;
            const d = workflowState.uiuxDiscovery;
            if (!d.answers) d.answers = {};
            d.answers[qid] = el.value;
        }

        function saveDesignAnswers() {
            saveState();
            intakeToast('디자인 답변이 저장되었습니다.');
        }

        function addReferenceSite() {
            const d = workflowState.uiuxDiscovery;
            if (!d.referenceSites) d.referenceSites = [];
            d.referenceSites.push({ id: 'rs-' + Date.now().toString(36), url: '', note: '', whatToFollow: '', whatToAvoid: '' });
            saveState();
            renderIntakePanelIfOpen();
        }

        function addReferenceImage() {
            const d = workflowState.uiuxDiscovery;
            if (!d.referenceImages) d.referenceImages = [];
            // metadata only: label/note/source/reflectedScope. No file bytes.
            d.referenceImages.push({ id: 'ri-' + Date.now().toString(36), label: '', note: '', source: 'user', reflectedScope: '' });
            saveState();
            renderIntakePanelIfOpen();
        }

        function editReference(kind, id, field, value) {
            const d = workflowState.uiuxDiscovery;
            const list = kind === 'site' ? (d.referenceSites || []) : (d.referenceImages || []);
            const item = list.find(i => i.id === id);
            if (item) item[field] = value;
        }

        function removeReference(kind, id) {
            const d = workflowState.uiuxDiscovery;
            if (kind === 'site') d.referenceSites = (d.referenceSites || []).filter(i => i.id !== id);
            else d.referenceImages = (d.referenceImages || []).filter(i => i.id !== id);
            saveState();
            renderIntakePanelIfOpen();
        }

        function commitReferences() {
            saveState();
            intakeToast('레퍼런스 메타데이터가 저장되었습니다.');
        }

        function setDesignApproval(status) {
            workflowState.uiuxDiscovery.designApprovalStatus = status;
            if (status === 'approved') workflowState.uiuxDiscovery.approvedAt = new Date().toISOString();
            saveState();
            renderIntakePanelIfOpen();
        }

        function onRejectedStylesInput(el) {
            workflowState.uiuxDiscovery.rejectedStyles = el.value.split('\n').map(s => s.trim()).filter(Boolean);
        }

        function esc2(s) { return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }

        function designQCardHtml(q) {
            const ans = (workflowState.uiuxDiscovery.answers || {})[q.id] || '';
            const badge = q.required ? '<span class="q-req">필수</span>' : '<span class="q-opt">선택</span>';
            const field = q.type === 'text'
                ? `<input class="q-input" type="text" data-qid="${q.id}" value="${esc2(ans)}" oninput="onDesignAnswerInput(this)" placeholder="답변">`
                : `<textarea class="q-input" data-qid="${q.id}" rows="2" oninput="onDesignAnswerInput(this)" placeholder="답변">${esc2(ans)}</textarea>`;
            return `<div class="q-card" data-required="${q.required}"><div class="q-head"><span class="q-cat">${q.category}</span>${badge}</div><div class="q-text">${esc2(q.text)}</div>${field}</div>`;
        }

        function refSiteHtml(r) {
            return `<div class="ref-item"><div class="up-row"><input class="up-name" value="${esc2(r.url)}" placeholder="참고 사이트 URL" oninput="editReference('site','${r.id}','url',this.value)"><button class="up-del" onclick="removeReference('site','${r.id}')">✕</button></div><input class="up-why" value="${esc2(r.note)}" placeholder="메모 (따라할 점 / 피할 점)" oninput="editReference('site','${r.id}','note',this.value)"></div>`;
        }

        function refImgHtml(r) {
            return `<div class="ref-item"><div class="up-row"><input class="up-name" value="${esc2(r.label)}" placeholder="이미지 라벨" oninput="editReference('img','${r.id}','label',this.value)"><button class="up-del" onclick="removeReference('img','${r.id}')">✕</button></div><input class="up-why" value="${esc2(r.reflectedScope)}" placeholder="반영 범위 (원본 복제 아님, 원칙만 추출)" oninput="editReference('img','${r.id}','reflectedScope',this.value)"></div>`;
        }

        // renderDesignRounds returns the full design-discovery block: round-1
        // questions, reference metadata, and the rejected re-ask shell.
        function renderDesignRounds() {
            const d = workflowState.uiuxDiscovery || {};
            const rounds = d.questionRounds || [];
            if (rounds.length === 0) {
                return `<div class="q-block"><button class="intake-btn active" onclick="generateDesignQuestions()">디자인 질문 생성</button>${intakeEmpty('디자인 취향 질문을 생성해 1차 라운드를 시작하세요.')}</div>`;
            }
            const qHtml = (rounds[0].questions || []).map(designQCardHtml).join('');
            const sites = (d.referenceSites || []).map(refSiteHtml).join('');
            const imgs = (d.referenceImages || []).map(refImgHtml).join('');
            const rejected = d.designApprovalStatus === 'rejected'
                ? `<div class="q-block"><h4 class="q-subtitle">3차 재질문 (시안 반려됨)</h4>${intakeEmpty('반려 사유를 반영해 다시 질문 라운드를 생성하세요.')}<button class="intake-btn active" onclick="generateDesignQuestions()">재질문 생성</button></div>`
                : '';
            return `<div class="q-block">
                <div class="q-toolbar"><button class="intake-btn active" onclick="generateDesignQuestions()">질문 다시 생성</button><button class="intake-btn active" onclick="saveDesignAnswers()">답변 저장</button></div>
                <h4 class="q-subtitle">1차 · 디자인 취향</h4>${qHtml}
                <h4 class="q-subtitle">2차 · 레퍼런스 (메타데이터만)</h4>
                <div class="q-toolbar"><button class="intake-btn active" onclick="addReferenceSite()">참고 사이트 추가</button><button class="intake-btn active" onclick="addReferenceImage()">참고 이미지 추가</button><button class="intake-btn active" onclick="commitReferences()">저장</button></div>
                ${sites || intakeEmpty('참고 사이트가 없습니다.')}${imgs}
                <h4 class="q-subtitle">싫어하는 스타일</h4>
                <textarea class="q-input" rows="2" placeholder="한 줄에 하나씩" oninput="onRejectedStylesInput(this)">${esc2((d.rejectedStyles || []).join('\n'))}</textarea>
                <div class="q-toolbar"><button class="intake-btn active" onclick="setDesignApproval('approved')">디자인 방향 승인</button><button class="intake-btn active" onclick="setDesignApproval('rejected')">반려</button></div>
                ${rejected}
            </div>`;
        }
