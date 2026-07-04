        function showPanel(title, result, agentId, opts = {}) {
            currentPreviewFile = null;
            panelAgentId = agentId || null;
            panelResult = result;
            const content = document.getElementById('panel-content');
            content.style.padding = '';
            content.style.whiteSpace = 'normal';
            document.getElementById('panel-title').innerText = title;
            const sentPrompt = agentId ? (getNodeState(agentId).lastPrompt || '') : '';
            const sentHtml = sentPrompt ? `<details style="margin-bottom:12px;border:1px solid var(--border-color);border-radius:6px;padding:6px 10px;background:var(--bg-secondary)"><summary style="cursor:pointer;font-size:12px;color:var(--text-dim);user-select:none">📨 보낸 메세지 보기</summary><pre style="margin:8px 0 0;white-space:pre-wrap;word-break:break-all;font-size:12px;color:var(--text-primary)">${sentPrompt.replace(/</g,'&lt;').replace(/>/g,'&gt;')}</pre></details>` : '';
            content.innerHTML = sentHtml + renderMarkdown(formatResult(result));
            if (typeof hljs !== 'undefined') content.querySelectorAll('pre code').forEach(block => hljs.highlightElement(block));
            content.querySelectorAll('.decision-btn').forEach(btn => { btn.onclick = () => applyDecision(btn); });
            const hasAgent = !!agentId;
            document.getElementById('approval-footer').style.display = (hasAgent && !!workflowState.approval.pendingNodeId) ? 'flex' : 'none';
            const forwardFooter = document.getElementById('forward-footer');
            if (opts.forwardFrom) {
                forwardFooter.dataset.from = opts.forwardFrom;
                forwardFooter.style.display = 'flex';
            } else {
                forwardFooter.style.display = 'none';
            }
            document.getElementById('overlay-panel').style.display = 'flex';
        }

        function closePanel() {
            document.getElementById('overlay-panel').style.display = 'none';

            currentPreviewFile = null;
            panelAgentId = null;
            panelResult = null;
        }

        (function initPanelDrag() {
            const panel = document.getElementById('overlay-panel');
            const header = document.getElementById('panel-header');
            let dragging = false, resizing = false, resizeDir = '';
            let ox = 0, oy = 0, startW = 0, startH = 0, startL = 0, startT = 0, startMX = 0, startMY = 0;

            const STORAGE_KEY = 'autopus-panel-layout';

            function savePanelLayout() {
                const layout = {
                    left: panel.style.left,
                    top: panel.style.top,
                    width: panel.style.width,
                    height: panel.style.height
                };
                localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
            }

            function restorePanelLayout() {
                try {
                    const raw = localStorage.getItem(STORAGE_KEY);
                    if (!raw) return;
                    const layout = JSON.parse(raw);
                    if (layout.left)   panel.style.left   = layout.left;
                    if (layout.top)    panel.style.top    = layout.top;
                    if (layout.width)  panel.style.width  = layout.width;
                    if (layout.height) panel.style.height = layout.height;
                } catch (_) {}
            }

            restorePanelLayout();

            header.addEventListener('mousedown', (e) => {
                if (e.target.tagName === 'BUTTON') return;
                dragging = true;
                const r = panel.getBoundingClientRect();
                ox = e.clientX - r.left;
                oy = e.clientY - r.top;
                header.style.cursor = 'grabbing';
                e.preventDefault();
            });

            panel.querySelectorAll('.panel-resize').forEach(handle => {
                handle.addEventListener('mousedown', (e) => {
                    resizing = true;
                    resizeDir = handle.dataset.dir;
                    const r = panel.getBoundingClientRect();
                    startW = panel.offsetWidth;
                    startH = panel.offsetHeight;
                    startL = r.left;
                    startT = r.top;
                    startMX = e.clientX;
                    startMY = e.clientY;
                    e.preventDefault();
                    e.stopPropagation();
                });
            });

            document.addEventListener('mousemove', (e) => {
                if (dragging) {
                    const maxLeft = Math.max(0, window.innerWidth  - panel.offsetWidth);
                    const maxTop  = Math.max(0, window.innerHeight - panel.offsetHeight);
                    panel.style.left = Math.min(Math.max(0, e.clientX - ox), maxLeft) + 'px';
                    panel.style.top  = Math.min(Math.max(0, e.clientY - oy), maxTop)  + 'px';
                }
                if (resizing) {
                    const dx = e.clientX - startMX;
                    const dy = e.clientY - startMY;
                    if (resizeDir.includes('e')) panel.style.width  = Math.min(Math.max(320, startW + dx), window.innerWidth  - startL) + 'px';
                    if (resizeDir.includes('s')) panel.style.height = Math.min(Math.max(200, startH + dy), window.innerHeight - startT) + 'px';
                    if (resizeDir.includes('w')) {
                        const newW = Math.max(320, startW - dx);
                        panel.style.width = newW + 'px';
                        panel.style.left = Math.max(0, startL + startW - newW) + 'px';
                    }
                    if (resizeDir.includes('n')) {
                        const newH = Math.max(200, startH - dy);
                        panel.style.height = newH + 'px';
                        panel.style.top = Math.max(0, startT + startH - newH) + 'px';
                    }
                }
            });

            document.addEventListener('mouseup', () => {
                if (dragging || resizing) savePanelLayout();
                if (dragging) header.style.cursor = 'grab';
                dragging = false;
                resizing = false;
                resizeDir = '';
            });
        })();

        async function connectProvider(provider) {
            try {
                const res = await fetch('/api/providers/connect', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ provider })
                });
                if (!res.ok) {
                    const message = await res.text();
                    appendTerminalLog('System', `[${provider}] 연결 확인 실패: ${message}`);
                    return;
                }
                const data = await res.json();
                appendTerminalLog('System', `[${provider}] ${data.message || 'CLI가 감지되었습니다.'}`);
                setTimeout(updateProviders, 1000);
            } catch (e) {
                appendTerminalLog('System', `[${provider}] 연결 확인 실패: ${e.message}`);
            }
        }

        async function saveState() {
            syncNodePositions();
            workflowState.version = 2;
            workflowState.systemPrompts = Object.fromEntries(systemPrompts);
            if (workflowState.logs.length > 300) workflowState.logs = workflowState.logs.slice(-200);
            await fetch('/api/workflow/state', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(workflowState) });
        }

        async function loadState() {
            let data;
            try {
                const res = await fetch('/api/workflow/state');
                if (!res.ok) return;
                data = await res.json();
            } catch (_) { return; }
            workflowState.version = data.version || 2;
            workflowState.nodes = data.nodes || [];
            workflowState.connections = data.connections || [];
            workflowState.logs = data.logs || [];
            workflowState.approval = data.approval || {};
            if (typeof restoreIntakeState === 'function') restoreIntakeState(data);
            if (typeof restoreUiuxDiscoveryState === 'function') restoreUiuxDiscoveryState(data);
            if (data.systemPrompts) Object.entries(data.systemPrompts).forEach(([k, v]) => systemPrompts.set(k, v));
            workflowState.logs.forEach((entry) => { if (entry.id) seenEventIds.add(entry.id); });
            // Check which agents are still running on the server before resetting.
            // A browser refresh does NOT kill server goroutines, so we must not
            // blindly reset 'active' → 'error' for agents that are still running.
            let runningSet = new Set();
            try {
                const runRes = await fetch('/api/workflow/running');
                if (runRes.ok) {
                    const runData = await runRes.json();
                    (runData.running || []).forEach((id) => runningSet.add(id));
                }
            } catch (_) {}
            let interrupted = false;
            workflowState.nodes.forEach((n) => {
                if (n.status === 'active') {
                    if (runningSet.has(n.id)) {
                        // Goroutine still running — keep as active, do not reset.
                    } else {
                        n.status = 'error';
                        interrupted = true;
                    }
                }
            });
            const pending = workflowState.approval.pendingNodeId;
            if (pending) {
                const node = getNodeState(pending);
                lastResult = node.output || null;
            }
            initNodes();
            // Use rAF so nodes are laid out before calculating connection positions.
            requestAnimationFrame(() => requestAnimationFrame(() => drawConnections()));
            renderTerminalLogs();
            if (interrupted) {
                appendTerminalLog('System', '⚠️ 이전 작업이 중단되었습니다. 노드를 클릭해 다시 실행하세요.', false);
            }
        }

        async function publishWorkflowEvent(type, agentId, message) {
            await fetch('/api/workflow/event', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ type, agentId, message })
            });
        }

        function connectWorkflowStream() {
            if (sseController) sseController.abort();
            sseController = new AbortController();
            sseRetrying = false;

            fetch('/api/workflow/stream', { signal: sseController.signal })
                .then((res) => {
                    if (!res.ok) throw new Error('SSE ' + res.status);
                    sseRetryDelay = 2000;
                    const reader = res.body.getReader();
                    const decoder = new TextDecoder();
                    let buf = '';
                    function pump() {
                        return reader.read().then(({ done, value }) => {
                            if (done) throw new Error('ended');
                            buf += decoder.decode(value, { stream: true });
                            let i;
                            while ((i = buf.indexOf('\n\n')) !== -1) {
                                parseSSEChunk(buf.slice(0, i));
                                buf = buf.slice(i + 2);
                            }
                            return pump();
                        });
                    }
                    return pump();
                })
                .catch((e) => {
                    if (e.name === 'AbortError') return;
                    if (!sseRetrying) {
                        appendTerminalLog('System', 'SSE 연결 끊김 — 자동 재연결 대기 중...', false);
                        sseRetrying = true;
                    }
                    sseRetryDelay = Math.min(sseRetryDelay * 2, 30000);
                    setTimeout(connectWorkflowStream, sseRetryDelay);
                });
        }

        function parseSSEChunk(chunk) {
            let data = '';
            for (const line of chunk.split('\n')) {
                if (line.startsWith('data:')) data = line.slice(5).trim();
            }
            if (!data) return;
            try { appendStreamEvent(JSON.parse(data), true); } catch (_) {}
        }

        function persistAgentFiles() {
            try { sessionStorage.setItem('agentFiles', JSON.stringify(Array.from(agentFiles.entries()).map(([k,v]) => [k, Array.from(v)]))); } catch(_) {}
        }

        function toggleSysPrompt() {
            const body = document.getElementById('sys-prompt-body');
            body.style.display = body.style.display === 'none' ? 'block' : 'none';
        }

        function saveSysPromptInMap() {
            if (!selectedAgent) return;
            const v = document.getElementById('sys-prompt-input').value;
            v ? systemPrompts.set(selectedAgent.id, v) : systemPrompts.delete(selectedAgent.id);
            document.getElementById('sys-prompt-badge').innerText = v ? ' ✓' : '';
        }

