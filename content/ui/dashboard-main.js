        async function handleFileUploadToServer(event) {
            if (!selectedAgent) { event.target.value = ''; return; }
            const files = event.target.files;
            if (!files.length) return;
            const fd = new FormData();
            Array.from(files).forEach(f => fd.append('files', f));
            try {
                const res = await fetch('/api/files/upload', { method: 'POST', body: fd });
                const data = await res.json();
                if (!agentFiles.has(selectedAgent.id)) agentFiles.set(selectedAgent.id, new Set());
                data.uploaded.forEach(name => agentFiles.get(selectedAgent.id).add(name));
                persistAgentFiles();
                await loadFiles();
                appendTerminalLog('System', `파일 업로드 완료: ${data.uploaded.join(', ')}`);
            } catch(e) { appendTerminalLog('System', '파일 업로드 실패: ' + ((e && e.message) ? e.message : '알 수 없는 오류')); }
            event.target.value = '';
        }

        function exportWorkflow() {
            syncNodePositions();
            workflowState.systemPrompts = Object.fromEntries(systemPrompts);
            const blob = new Blob([JSON.stringify(workflowState, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'workflow-' + new Date().toISOString().slice(0,10) + '.json';
            document.body.appendChild(a); a.click(); document.body.removeChild(a);
            URL.revokeObjectURL(url);
        }

        async function importWorkflow(event) {
            const file = event.target.files[0];
            if (!file) return;
            try {
                const data = JSON.parse(await file.text());
                workflowState.nodes = data.nodes || [];
                workflowState.connections = data.connections || [];
                workflowState.logs = data.logs || [];
                workflowState.approval = data.approval || {};
                systemPrompts.clear();
                if (data.systemPrompts) Object.entries(data.systemPrompts).forEach(([k,v]) => systemPrompts.set(k, v));
                initNodes(); requestAnimationFrame(() => requestAnimationFrame(() => drawConnections())); renderTerminalLogs();
                await saveState();
                appendTerminalLog('System', `워크플로우 불러오기: ${file.name}`);
            } catch(e) { alert('워크플로우 파일 오류: ' + e.message); }
            event.target.value = '';
        }

        function renderDecisionBlock(raw) {
            const lines = raw.split('\n').map(l => l.trim()).filter(Boolean);
            let title = '', desc = '', options = [], recommend = '';
            for (const line of lines) {
                if (line.startsWith('항목:')) title = line.slice(3).trim();
                else if (line.startsWith('설명:')) desc = line.slice(3).trim();
                else if (line.startsWith('옵션:')) options.push(line.slice(3).trim());
                else if (line.startsWith('추천:')) recommend = line.slice(3).trim();
            }
            if (!title && !options.length) return '';
            const esc = s => s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
            const btns = options.map(opt => {
                const dashIdx = opt.indexOf('—');
                const name = dashIdx > -1 ? opt.slice(0, dashIdx).trim() : opt.trim();
                const tip = dashIdx > -1 ? opt.slice(dashIdx + 1).trim() : '';
                const isRec = recommend && recommend.startsWith(name);
                return `<button class="decision-btn${isRec ? ' recommended' : ''}" data-choice="${esc(name)}" data-item="${esc(title)}" title="${esc(tip)}">${isRec ? '⭐ ' : ''}${esc(name)}</button>`;
            }).join('');
            return `<div class="decision-card" data-title="${esc(title)}"><div class="decision-header">🙋 ${esc(title)}</div>${desc ? `<div class="decision-desc">${esc(desc)}</div>` : ''}<div class="decision-options">${btns}</div></div>`;
        }

        function renderMarkdown(text) {
            if (!text) return '';
            try {
                if (typeof DOMPurify !== 'undefined' && typeof marked !== 'undefined') {
                    const renderer = new marked.Renderer();
                    const origCode = renderer.code.bind(renderer);
                    renderer.code = function(code, lang) {
                        if ((lang || '').trim() === 'decision') return renderDecisionBlock(code);
                        return origCode(code, lang);
                    };
                    return DOMPurify.sanitize(marked.parse(text, { renderer }));
                }
            } catch(_) {}
            const el = document.createElement('div');
            el.textContent = text;
            return '<pre style="white-space:pre-wrap">' + el.innerHTML + '</pre>';
        }

        function applyDecision(btn) {
            const choice = btn.dataset.choice;
            const itemTitle = btn.dataset.item;
            if (!choice || !panelAgentId) return;
            const agentId = panelAgentId;
            const ns = getNodeState(agentId);
            const decisionPrompt = `"${itemTitle}" 항목에서 "${choice}"을(를) 선택합니다.\n\n` +
                `관련 파일을 즉시 열어 이 결정 내용을 반영하고 ## 작업 요약 형식으로 보고하세요.\n` +
                `추가 질문이나 선택지 나열 없이 파일 수정 후 바로 종료하세요.`;
            closePanel();
            runAgent(agentId, decisionPrompt, {
                summary: `"${itemTitle}" → "${choice}" 선택`,
                output: ns && ns.output ? (ns.output.output || '') : ''
            }, (ns && ns.originalRequest) || decisionPrompt);
        }

        function filterFileList(query) {
            const q = (query || '').toLowerCase();
            document.querySelectorAll('#file-list .item').forEach(item => {
                item.style.display = (!q || item.textContent.toLowerCase().includes(q)) ? '' : 'none';
            });
        }

        function renderLogTabs() {
            const bar = document.getElementById('log-tab-bar');
            if (!bar) return;
            if (logAgents.size === 0) { bar.style.display = 'none'; return; }
            bar.style.display = 'flex';
            const btnStyle = (active) => `background:${active ? 'var(--accent-blue)' : 'transparent'};color:${active ? '#fff' : 'var(--text-dim)'};border:1px solid ${active ? 'var(--accent-blue)' : 'var(--border-color)'};padding:2px 10px;border-radius:12px;cursor:pointer;font-size:11px;`;
            bar.innerHTML = '';
            const makeBtn = (label, agentId) => {
                const btn = document.createElement('button');
                btn.style.cssText = btnStyle(activeLogTab === agentId);
                btn.textContent = label;
                btn.dataset.tabAgent = agentId;
                btn.addEventListener('click', () => filterLogsByTab(agentId));
                return btn;
            };
            bar.appendChild(makeBtn('전체', 'all'));
            logAgents.forEach(agent => bar.appendChild(makeBtn(agent, agent)));
        }

        function filterLogsByTab(agentId) {
            activeLogTab = agentId;
            renderLogTabs();
            document.querySelectorAll('#terminal > div').forEach(div => {
                div.style.display = (agentId === 'all' || div.dataset.agent === agentId) ? '' : 'none';
            });
        }

        document.addEventListener('keydown', function(e) {
            const tag = document.activeElement ? document.activeElement.tagName : '';
            if (tag === 'TEXTAREA' || tag === 'INPUT') return;
            const panelVisible = document.getElementById('overlay-panel').style.display === 'flex';
            const hasPending = !!workflowState.approval.pendingNodeId;
            if ((e.key === 'Enter' || (e.ctrlKey && e.key === 'Enter')) && panelVisible && hasPending) {
                e.preventDefault(); handleApprove();
            } else if (e.key === 'Escape' && panelVisible) {
                e.preventDefault(); closePanel();
            } else if (e.ctrlKey && !e.shiftKey && e.key === 'r') {
                e.preventDefault();
                if (selectedAgent) openDrawer(selectedAgent);
            } else if (e.ctrlKey && e.shiftKey && e.key === 'F') {
                e.preventDefault();
                const fi = document.getElementById('file-search-input');
                if (fi) fi.focus();
            }
        });

        async function init() {
            appendTerminalLog('System', '16인 완전체 팀 소환 완료. 정식판 v5.0 가동.', true);
            initNodes();
            renderRecentProjects();
            let startPath = '.';
            try {
                const recent = JSON.parse(localStorage.getItem('recentProjects') || '[]');
                if (recent.length > 0 && recent[0].path) startPath = recent[0].path;
            } catch(_) {}
            try {
                await changeWorkspace(startPath, true);
            } catch(_) {
                if (startPath !== '.') await changeWorkspace('.', true);
            }
            // Record the landing dir so it appears in recent projects
            try {
                const res = await fetch('/api/workspace/list');
                const d = await res.json();
                if (d.current) { pushRecentProject(d.current); renderRecentProjects(); }
            } catch(_) {}
            updateProviders();  // fire-and-forget: don't block page load on provider probe
            connectWorkflowStream();
            setInterval(updateProviders, 60000);  // 60s interval; server caches connected status for 5min
            document.getElementById('overlay-panel').addEventListener('wheel', (e) => {
                e.stopPropagation();
            }, { passive: true });
            document.getElementById('canvas-area').addEventListener('wheel', (e) => {
                e.preventDefault();
                if (e.deltaY < 0) zoomIn(); else zoomOut();
            }, { passive: false });
            const canvasArea = document.getElementById('canvas-area');
            let panStartX = 0, panStartY = 0;
            canvasArea.addEventListener('mousedown', (e) => {
                if (e.target.closest('.node') || e.target.closest('.port') || e.target.closest('#overlay-panel')) return;
                isPanning = true;
                panStartX = e.clientX - panX;
                panStartY = e.clientY - panY;
                canvasArea.classList.add('panning');
            });
            document.addEventListener('mousemove', (e) => {
                if (!isPanning) return;
                panX = e.clientX - panStartX;
                panY = e.clientY - panStartY;
                applyZoom();
            });
            document.addEventListener('mouseup', () => {
                if (isPanning) { isPanning = false; canvasArea.classList.remove('panning'); }
            });
        }

        init();

        async function shutdownServer() {
            if (!confirm('서버를 종료하시겠습니까?\n(autopus.exe가 중지됩니다)')) return;
            try { await saveState(); } catch(_) {}
            try { await fetch('/api/shutdown', { method: 'POST' }); } catch(_) {}
            document.body.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:monospace;color:#888;">서버가 종료되었습니다. 창을 닫아주세요.</div>';
        }

        // Expose functions for inline HTML event handlers
        window.assignTask = assignTask;
        window.cancelAgent = cancelAgent;
        window.changeWorkspace = changeWorkspace;
        window.clearConnections = clearConnections;
        window.closeDrawer = closeDrawer;
        window.closePanel = closePanel;
        window.connectProvider = connectProvider;
        window.exportWorkflow = exportWorkflow;
        window.filterFileList = filterFileList;
        window.forwardCompletedToNext = forwardCompletedToNext;
        window.handleApprove = handleApprove;
        window.handleFileUploadToServer = handleFileUploadToServer;
        window.handleReject = handleReject;
        window.handleRerunWithEdit = handleRerunWithEdit;
        window.importWorkflow = importWorkflow;
        window.loadFiles = loadFiles;
        window.onMonitorNodeClick = onMonitorNodeClick;
        window.reopenApprovalPanel = reopenApprovalPanel;
        window.resetNodeError = resetNodeError;
        window.saveApprovedDoc = saveApprovedDoc;
        window.saveState = saveState;
        window.saveSysPromptInMap = saveSysPromptInMap;
        window.shutdownServer = shutdownServer;
        window.switchMode = switchMode;
        window.togglePipelineMode = togglePipelineMode;
        window.toggleRecentProjects = toggleRecentProjects;
        window.toggleSysPrompt = toggleSysPrompt;
        window.toggleTheme = toggleTheme;
        window.zoomIn = zoomIn;
        window.zoomOut = zoomOut;
        window.autoFixAgent = autoFixAgent;
        window.viewCompletedOutput = viewCompletedOutput;
        window.openIntakePanel = openIntakePanel;
