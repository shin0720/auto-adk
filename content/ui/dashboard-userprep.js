        // PR #25: UserPrep checklist. Rule/keyword-based draft from readiness
        // answers + manual add/edit/delete. NEVER captures or stores secret
        // values (only item names/notes). Persists via workflowState.readiness.

        // Each rule maps readiness-answer keywords to a checklist category. The
        // draft only records THAT a preparation item is needed — never its value.
        const USERPREP_RULES = [
            { cat: 'API key',              kw: /api\s*key|api키|엑세스\s*키|access\s*key/i,        why: '외부 서비스 실연동에 필요' },
            { cat: 'OAuth client',         kw: /oauth|로그인|소셜\s*로그인|login|sso/i,             why: '소셜 로그인/인증 연동에 필요' },
            { cat: '개발자 계정',           kw: /개발자\s*계정|developer\s*account|앱\s*등록/i,       why: '외부 플랫폼 앱 등록에 필요' },
            { cat: '결제/요금제',           kw: /결제|payment|stripe|구독|요금제|billing/i,          why: '결제 흐름 실연동에 필요' },
            { cat: '도메인/DNS',           kw: /도메인|domain|dns/i,                               why: '배포 도메인 연결에 필요' },
            { cat: '데이터 파일',           kw: /데이터\s*파일|csv|시드\s*데이터|seed|엑셀|xlsx/i,   why: '초기 데이터 구성에 필요' },
            { cat: '이미지/로고/브랜드',     kw: /로고|logo|브랜드|brand|무드보드/i,                  why: '브랜드 자산 반영에 필요' },
            { cat: '개인정보처리방침/약관',  kw: /약관|개인정보|privacy|terms/i,                     why: '법적 고지 페이지에 필요' },
            { cat: '배포 계정',            kw: /배포|deploy|vercel|netlify|호스팅|hosting/i,       why: '배포 플랫폼 계정에 필요' },
            { cat: '테스트 계정',          kw: /테스트\s*계정|test\s*account|샌드박스|sandbox/i,    why: '연동 테스트에 필요' },
            { cat: '외부 서비스 권한',      kw: /권한|scope|permission|api\s*권한/i,                why: '외부 서비스 접근 권한에 필요' },
            { cat: '법적/라이선스',         kw: /라이선스|license|상용|commercial|저작권/i,          why: '상용 사용 적합성 확인에 필요' }
        ];

        function readinessAnswerText() {
            const ans = (workflowState.intake && workflowState.intake.answers) || {};
            return Object.keys(ans)
                .filter(k => k.startsWith('rd-'))
                .map(k => ans[k])
                .join('\n');
        }

        // generateUserPrepDraft scans readiness answers and appends checklist
        // items for matched categories (dedup by category). Existing items and
        // their status are preserved.
        function generateUserPrepDraft() {
            const text = readinessAnswerText();
            if (!workflowState.readiness.userPrepChecklist) workflowState.readiness.userPrepChecklist = [];
            const list = workflowState.readiness.userPrepChecklist;
            const have = new Set(list.map(i => i.name));
            let added = 0;
            USERPREP_RULES.forEach(rule => {
                if (rule.kw.test(text) && !have.has(rule.cat)) {
                    list.push({ id: 'up-' + rule.cat.replace(/[^a-zA-Z0-9]/g, '').slice(0, 8) + '-' + Date.now().toString(36), name: rule.cat, whyNeeded: rule.why, requiredForMVP: true, status: 'pending', fallback: '', blockingLevel: 'soft' });
                    added++;
                }
            });
            workflowState.readiness.status = 'classified';
            saveState();
            renderIntakePanelIfOpen();
            intakeToast(added > 0 ? `준비물 ${added}개 초안 생성` : '매칭된 준비물 키워드가 없습니다. 수동으로 추가하세요.');
        }

        function addUserPrepItem() {
            if (!workflowState.readiness.userPrepChecklist) workflowState.readiness.userPrepChecklist = [];
            workflowState.readiness.userPrepChecklist.push({ id: 'up-manual-' + Date.now().toString(36), name: '새 준비 항목', whyNeeded: '', requiredForMVP: false, status: 'pending', fallback: '', blockingLevel: 'none' });
            saveState();
            renderIntakePanelIfOpen();
        }

        function removeUserPrepItem(id) {
            const list = workflowState.readiness.userPrepChecklist || [];
            workflowState.readiness.userPrepChecklist = list.filter(i => i.id !== id);
            saveState();
            renderIntakePanelIfOpen();
        }

        function editUserPrepField(id, field, value) {
            const item = (workflowState.readiness.userPrepChecklist || []).find(i => i.id === id);
            if (!item) return;
            item[field] = value;
            // No saveState on every keystroke; status change (select) saves below.
        }

        function commitUserPrep() {
            saveState();
            intakeToast('준비물 체크리스트가 저장되었습니다.');
        }

        function userPrepItemHtml(item) {
            const esc = s => (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
            const opt = st => `<option value="${st}"${item.status === st ? ' selected' : ''}>${st}</option>`;
            return `<div class="up-item"><div class="up-row"><input class="up-name" value="${esc(item.name)}" oninput="editUserPrepField('${item.id}','name',this.value)"><select class="up-status" onchange="editUserPrepField('${item.id}','status',this.value);commitUserPrep()">${opt('pending')}${opt('provided')}${opt('skipped')}</select><button class="up-del" title="삭제" onclick="removeUserPrepItem('${item.id}')">✕</button></div><input class="up-why" value="${esc(item.whyNeeded)}" placeholder="필요한 이유" oninput="editUserPrepField('${item.id}','whyNeeded',this.value)"></div>`;
        }

        // renderUserPrep returns the checklist block. Secret values are never
        // requested here — only the human-readable name and reason.
        function renderUserPrep() {
            const list = (workflowState.readiness && workflowState.readiness.userPrepChecklist) || [];
            const items = list.map(userPrepItemHtml).join('');
            const body = list.length === 0
                ? intakeEmpty('Readiness 답변을 입력한 뒤 "준비물 초안 생성"을 누르거나, 항목을 직접 추가하세요.')
                : items;
            return `<div class="up-block"><div class="q-toolbar"><button class="intake-btn active" onclick="generateUserPrepDraft()">준비물 초안 생성</button><button class="intake-btn active" onclick="addUserPrepItem()">항목 추가</button><button class="intake-btn active" onclick="commitUserPrep()">저장</button></div><p class="up-note">🔒 API 키·비밀번호 등 실제 값은 여기에 입력하지 마세요. 항목 이름만 기록합니다.</p>${body}</div>`;
        }
