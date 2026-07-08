        function startDragging(event, el) {
            const canvasRect = document.getElementById('canvas-area').getBoundingClientRect();
            const shiftX = event.clientX - el.getBoundingClientRect().left;
            const shiftY = event.clientY - el.getBoundingClientRect().top;
            document.onmousemove = (moveEvent) => {
                el.style.left = (moveEvent.clientX - canvasRect.left - panX - shiftX) / zoomLevel + 'px';
                el.style.top = (moveEvent.clientY - canvasRect.top - panY - shiftY) / zoomLevel + 'px';
                drawConnections();
            };
            document.onmouseup = () => { document.onmousemove = null; saveState(); };
        }

        function startConnection(event, fromId) {
            event.stopPropagation();
            isConnecting = true;
            tempLineFrom = fromId;
            document.onmousemove = drawTempLine;
            document.onmouseup = () => {
                isConnecting = false;
                document.onmousemove = null;
                document.getElementById('flow-svg').querySelector('.temp-line')?.remove();
            };
        }

        function endConnection(event, toId) {
            if (!isConnecting || tempLineFrom === toId) return;
            workflowState.connections.push({ from: tempLineFrom, to: toId });
            drawConnections();
            saveState();
            if (typeof refreshMonitorIfActive === 'function') refreshMonitorIfActive();
        }

        function drawConnections() {
            const svg = document.getElementById('flow-svg');
            const defs = svg.querySelector('defs').outerHTML;
            svg.innerHTML = defs;
            const canvasRect = document.getElementById('canvas-area').getBoundingClientRect();
            workflowState.connections.forEach((conn, index) => {
                const from = document.getElementById('node-' + conn.from);
                const to = document.getElementById('node-' + conn.to);
                if (!from || !to) return;
                const fromRect = from.getBoundingClientRect();
                const toRect = to.getBoundingClientRect();
                const x1 = (fromRect.right - canvasRect.left - panX) / zoomLevel;
                const y1 = (fromRect.top + fromRect.height / 2 - canvasRect.top - panY) / zoomLevel;
                const x2 = (toRect.left - canvasRect.left - panX) / zoomLevel;
                const y2 = (toRect.top + toRect.height / 2 - canvasRect.top - panY) / zoomLevel;
                const mx = (x1 + x2) / 2;
                const my = (y1 + y2) / 2;
                const d = `M ${x1} ${y1} C ${x1 + 40} ${y1}, ${x2 - 40} ${y2}, ${x2} ${y2}`;

                const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                g.setAttribute('class', 'flow-group');

                // Wide transparent hit area for easier hover
                const hit = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                hit.setAttribute('d', d);
                hit.setAttribute('fill', 'none');
                hit.setAttribute('stroke', 'transparent');
                hit.setAttribute('stroke-width', '16');
                hit.setAttribute('pointer-events', 'auto');
                g.appendChild(hit);

                const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                path.setAttribute('d', d);
                path.setAttribute('class', 'flow-line');
                path.setAttribute('marker-end', 'url(#arrow)');
                g.appendChild(path);

                // X delete button at midpoint
                const delG = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                delG.setAttribute('class', 'flow-delete');

                const circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
                circle.setAttribute('cx', mx);
                circle.setAttribute('cy', my);
                circle.setAttribute('r', '9');
                circle.setAttribute('fill', 'var(--err)');
                circle.setAttribute('stroke', 'var(--bg)');
                circle.setAttribute('stroke-width', '2');

                const xText = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                xText.setAttribute('x', mx);
                xText.setAttribute('y', my);
                xText.setAttribute('text-anchor', 'middle');
                xText.setAttribute('dominant-baseline', 'central');
                xText.setAttribute('fill', 'white');
                xText.setAttribute('font-size', '12');
                xText.setAttribute('font-weight', 'bold');
                xText.setAttribute('pointer-events', 'none');
                xText.textContent = '×';

                delG.appendChild(circle);
                delG.appendChild(xText);
                delG.onclick = (e) => {
                    e.stopPropagation();
                    workflowState.connections.splice(index, 1);
                    drawConnections();
                    saveState();
                    if (typeof refreshMonitorIfActive === 'function') refreshMonitorIfActive();
                };
                g.appendChild(delG);
                svg.appendChild(g);
            });
        }

        function drawTempLine(event) {
            const svg = document.getElementById('flow-svg');
            svg.querySelector('.temp-line')?.remove();
            const from = document.getElementById('node-' + tempLineFrom).getBoundingClientRect();
            const canvasRect = document.getElementById('canvas-area').getBoundingClientRect();
            const line = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            line.setAttribute('d', `M ${(from.right - canvasRect.left - panX) / zoomLevel} ${(from.top + from.height / 2 - canvasRect.top - panY) / zoomLevel} L ${(event.clientX - canvasRect.left - panX) / zoomLevel} ${(event.clientY - canvasRect.top - panY) / zoomLevel}`);
            line.setAttribute('class', 'temp-line');
            svg.appendChild(line);
        }

        function clearConnections() {
            workflowState.connections = [];
            drawConnections();
            saveState();
            if (typeof refreshMonitorIfActive === 'function') refreshMonitorIfActive();
        }

        async function updateProviders() {
            try {
                const controller = new AbortController();
                const tid = setTimeout(() => controller.abort(), 8000);
                const res = await fetch('/api/providers/status', { signal: controller.signal });
                clearTimeout(tid);
                if (!res.ok) return;
                const raw = await res.json();
                const providers = Array.isArray(raw)
                    ? raw
                    : Object.entries(raw).map(([id, connected]) => ({ id, connected }));
                providerStatuses = providers;
                if (selectedAgent) renderProviderCheckboxes(selectedAgent.id);
                providers.forEach((provider) => {
                    const dot = document.getElementById('dot-' + provider.id);
                    const label = document.getElementById('provider-' + provider.id);
                    const btn = document.getElementById('btn-' + provider.id);
                    // PR #29: reflect the hardened authState — 'connected' alone
                    // no longer implies the provider can actually run.
                    const aState = provider.authState || (provider.connected ? 'available' : 'unavailable');
                    const dotClass = aState === 'available' ? 'online' : aState === 'unavailable' ? 'offline' : 'auth';
                    if (dot) dot.className = 'dot ' + dotClass;
                    if (label) label.innerText = provider.statusLabel
                        ? provider.statusLabel + (provider.connected && lastPromptTokens > 0 ? ` · ~${lastPromptTokens.toLocaleString()}토큰` : '')
                        : (provider.connected ? (provider.version || 'connected') : (provider.issue || 'CLI 실행 불가'));
                    if (btn) btn.style.display = provider.connected ? 'none' : '';
                });
            } catch (e) {
                console.warn('[Autopus] provider status 조회 실패:', e.message);
            }
        }

        function updateTokenCount() {
            const text = document.getElementById('prompt-input').value;
            document.getElementById('token-count').innerText = Math.ceil(text.length / 3.5).toLocaleString();
        }

        function renderProviderCheckboxes(agentId) {
            const container = document.getElementById('provider-checkboxes');
            if (!container) return;
            const selected = selectedProviders.get(agentId) || new Set();
            const useDefault = selected.size === 0;
            container.innerHTML = '';
            if (!providerStatuses.length) {
                container.innerHTML = '<div style="font-size:11px;color:var(--text-dim);padding:4px 0;">프로바이더 정보 로딩 중...</div>';
                return;
            }
            providerStatuses.forEach((p) => {
                const row = document.createElement('label');
                row.className = 'provider-cb-row';
                const cb = document.createElement('input');
                cb.type = 'checkbox';
                cb.checked = useDefault ? p.id === (agentDefaultProviders[agentId] || 'claude') : selected.has(p.id);
                cb.onchange = () => {
                    if (!selectedProviders.has(agentId)) selectedProviders.set(agentId, new Set());
                    cb.checked ? selectedProviders.get(agentId).add(p.id) : selectedProviders.get(agentId).delete(p.id);
                };
                const name = document.createElement('span');
                name.innerText = p.name || p.id;
                const badge = document.createElement('span');
                const cbState = p.authState || (p.connected ? 'available' : 'unavailable');
                badge.className = 'provider-cb-status ' + (cbState === 'available' ? 'online' : cbState === 'unavailable' ? 'offline' : 'auth');
                badge.innerText = p.statusLabel || (p.connected ? '연결됨' : '오프라인');
                if (p.isOptionalSupport) badge.title = '선택 보조 · 없어도 진행 가능';
                row.appendChild(cb);
                row.appendChild(name);
                row.appendChild(badge);
                container.appendChild(row);
            });
        }

        function selectNode(agent) {
            document.querySelectorAll('.node.selected').forEach(n => n.classList.remove('selected'));
            const el = document.getElementById('node-' + agent.id);
            if (el) el.classList.add('selected');
            selectedAgent = agent;
        }

        function openDrawer(agent, presetPrompt = '') {
            selectNode(agent);
            document.getElementById('drawer-title').innerText = agent.name;
            const ns = getNodeState(agent.id);
            document.getElementById('drawer-desc').innerText = ns.rerunContext
                ? `이전 결과를 참고하여 수정 지시를 입력하세요. (이전 결과 보기 버튼으로 확인 가능)`
                : `${agent.role}에게 전달할 지시를 작성하세요.`;
            const outputBtnArea = document.getElementById('drawer-output-btn-area');
            if (outputBtnArea) {
                const nodeState = getNodeState(agent.id);
                if (nodeState.output) {
                    outputBtnArea.style.display = 'block';
                    outputBtnArea.innerHTML = '';
                    const btn = document.createElement('button');
                    btn.textContent = '📋 이전 결과 보기';
                    btn.style.cssText = 'width:100%;padding:7px;background:transparent;border:1px solid var(--accent-blue);color:var(--accent-blue);border-radius:4px;font-size:12px;cursor:pointer;';
                    btn.onmouseenter = () => { btn.style.background = 'rgba(88,166,255,0.08)'; };
                    btn.onmouseleave = () => { btn.style.background = 'transparent'; };
                    btn.onclick = () => viewCompletedOutput(agent.id);
                    outputBtnArea.appendChild(btn);
                } else {
                    outputBtnArea.style.display = 'none';
                    outputBtnArea.innerHTML = '';
                }
            }
            const fileSet = agentFiles.get(agent.id) || new Set();
            const drawerFiles = document.getElementById('drawer-files');
            if (fileSet.size > 0) {
                drawerFiles.style.display = 'block';
                drawerFiles.innerHTML = `<div style="font-size:11px;color:var(--text-dim);margin-bottom:6px;font-weight:bold;">📎 배정된 파일 (${fileSet.size}개)</div>`;
                fileSet.forEach(f => {
                    const row = document.createElement('div');
                    row.style.cssText = 'display:flex;align-items:center;gap:6px;padding:3px 0;';
                    const lbl = document.createElement('span');
                    lbl.style.cssText = 'flex:1;font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--fg);';
                    lbl.innerText = '📄 ' + f;
                    const rmBtn = document.createElement('button');
                    rmBtn.innerText = '✕';
                    rmBtn.title = '파일 제거';
                    rmBtn.style.cssText = 'background:transparent;border:none;color:var(--err);cursor:pointer;padding:2px 5px;font-size:12px;line-height:1;flex-shrink:0;';
                    rmBtn.onclick = () => {
                        agentFiles.get(agent.id).delete(f);
                        persistAgentFiles();
                        openDrawer(agent, document.getElementById('prompt-input').value);
                    };
                    row.appendChild(lbl);
                    row.appendChild(rmBtn);
                    drawerFiles.appendChild(row);
                });
            } else {
                drawerFiles.style.display = 'none';
                drawerFiles.innerHTML = '';
            }
            const ta = document.getElementById('prompt-input');
            ta.value = presetPrompt;
            ta.oninput = updateTokenCount;
            updateTokenCount();
            renderProviderCheckboxes(agent.id);
            document.getElementById('sys-prompt-input').value = systemPrompts.get(agent.id) || '';
            document.getElementById('sys-prompt-badge').innerText = systemPrompts.has(agent.id) ? ' ✓' : '';
            document.getElementById('sys-prompt-body').style.display = 'none';
            document.getElementById('sidebar-right').style.display = 'flex';
            loadFiles(); // refresh checkboxes for this agent
        }

        function closeDrawer() { document.getElementById('sidebar-right').style.display = 'none'; }

