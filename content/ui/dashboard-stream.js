        function formatResult(result) {
            if (!result) return '';
            const files = (result.contextFiles || []).length ? '\n\n컨텍스트 파일:\n- ' + result.contextFiles.join('\n- ') : '';
            return `요약: ${result.summary || ''}\n에이전트: ${result.fromAgent || ''}\n시각: ${result.timestamp || ''}${files}\n\n${result.output || ''}`;
        }

        function appendTerminalLog(agent, message, persist = true) {
            const timestamp = new Date().toLocaleTimeString('ko-KR', { hour12: false });
            const entry = { agent, message, timestamp };
            const terminal = document.getElementById('terminal');
            const div = document.createElement('div');
            div.dataset.agent = agent;
            div.innerHTML = `<strong>[${entry.timestamp}] [${entry.agent}]</strong> ${entry.message}`;
            if (activeLogTab !== 'all' && activeLogTab !== agent) div.style.display = 'none';
            while (terminal.children.length > 300) terminal.removeChild(terminal.firstChild);
            terminal.appendChild(div);
            terminal.scrollTop = 9999;
            if (persist) workflowState.logs.push(entry);
            if (agent !== 'System') { logAgents.add(agent); renderLogTabs(); }
        }

        // clearTerminalLogs clears ONLY the terminal log history. It is invoked
        // solely by an explicit user click on the "로그 지우기" button — never on
        // page load, workflow start/finish, or provider status refresh. It does NOT
        // touch workflow nodes/outputs, connections, approval, planningCouncil,
        // provider results, files, or canvas layout — only workflowState.logs.
        function clearTerminalLogs() {
            if (!confirm('터미널 로그만 지울까요? 작업 결과·파일·승인 내역은 삭제되지 않습니다.')) return;
            workflowState.logs = [];
            const terminal = document.getElementById('terminal');
            if (terminal) terminal.innerHTML = '';
            logAgents.clear();
            activeLogTab = 'all';
            if (typeof renderLogTabs === 'function') renderLogTabs();
            if (typeof saveState === 'function') saveState();
        }

        function showStreamingChunks(agentId, agentName, fullText) {
            const old = chunkQueues.get(agentId);
            if (old) { clearTimeout(old.timer); chunkQueues.delete(agentId); }
            const terminal = document.getElementById('terminal');
            const wrapper = document.createElement('div');
            wrapper.dataset.agent = agentName;
            if (activeLogTab !== 'all' && activeLogTab !== agentName) wrapper.style.display = 'none';
            const ts = new Date().toLocaleTimeString('ko-KR', { hour12: false });
            const header = document.createElement('div');
            header.innerHTML = `<strong>[${ts}] [${agentName}]</strong> 응답:`;
            wrapper.appendChild(header);
            const pre = document.createElement('pre');
            pre.style.cssText = 'margin:4px 0 6px 0;padding:6px 10px;background:rgba(255,255,255,0.04);border-left:2px solid var(--accent);font-size:0.82em;white-space:pre-wrap;word-break:break-word;max-height:420px;overflow-y:auto;line-height:1.55;';
            wrapper.appendChild(pre);
            terminal.appendChild(wrapper);
            logAgents.add(agentName); renderLogTabs();
            const lines = fullText.split('\n').filter(l => l !== '');
            const state = { wrapper, pre, lines, index: 0, timer: null };
            chunkQueues.set(agentId, state);
            function drainNext() {
                if (state.index >= state.lines.length) { state.timer = null; return; }
                const line = state.lines[state.index++];
                pre.textContent += (pre.textContent ? '\n' : '') + line;
                pre.scrollTop = pre.scrollHeight;
                terminal.scrollTop = 9999;
                state.timer = setTimeout(drainNext, 35);
            }
            drainNext();
        }

        async function appendStreamEvent(event, persist = true) {
            if (!event || !event.id || seenEventIds.has(event.id)) return;
            seenEventIds.add(event.id);
            if (seenEventIds.size > 500) {
                const iter = seenEventIds.values();
                for (let i = 0; i < 100; i++) seenEventIds.delete(iter.next().value);
            }
            if (event.type === 'chunk' && event.agentId && event.message) {
                showStreamingChunks(event.agentId, event.agentName || event.agentId || 'System', event.message);
                return;
            }
            if (event.type === 'working' && event.agentId) {
                // Transient heartbeat — show in terminal but don't persist to log
                const agentLabel = (agentMap[event.agentId] || {}).name || event.agentName || event.agentId;
                const displayTime = new Date(event.timestamp || Date.now()).toLocaleTimeString('ko-KR', { hour12: false });
                const div = document.createElement('div');
                div.dataset.agent = agentLabel;
                div.style.color = 'var(--fg2)';
                div.innerHTML = `<strong>[${displayTime}] [${agentLabel}]</strong> ${event.message}`;
                document.getElementById('terminal').appendChild(div);
                document.getElementById('terminal').scrollTop = 9999;
                return;
            }
            if (event.type === 'checklist' && event.agentId && event.checklist) {
                updateNodeChecklist(event.agentId, event.checklist);
                return;
            }
            if (event.type === 'result' && event.agentId && event.result) {
                recoveryAttempts = 0;
                lastResult = event.result;
                const node = getNodeState(event.agentId);
                node.output = event.result;
                const agentObj = agentMap[event.agentId];
                const agentLabel = agentObj ? agentObj.name : (event.agentName || event.agentId);
                if (pipelineAutoMode) {
                    setNodeStatus(event.agentId, 'completed');
                    workflowState.approval = { pendingNodeId: '', lastDecision: 'auto' };
                    saveState();
                    await autoAdvancePipeline(event.agentId, event.result);
                } else {
                    setNodeStatus(event.agentId, 'awaiting-approval');
                    workflowState.approval = { pendingNodeId: event.agentId, lastDecision: '' };
                    showPanel(agentLabel + ' 분석 결과', event.result, event.agentId);
                    saveState();
                }
                return;
            }
            const entry = {
                id: event.id,
                type: event.type || '',
                agentId: event.agentId || '',
                agent: event.agentName || event.agentId || 'System',
                message: event.message || '',
                timestamp: event.timestamp || new Date().toISOString()
            };
            const displayTime = new Date(entry.timestamp).toLocaleTimeString('ko-KR', { hour12: false });
            const div = document.createElement('div');
            div.dataset.agent = entry.agent;
            div.innerHTML = `<strong>[${displayTime}] [${entry.agent}]</strong> ${entry.message}`;
            if (activeLogTab !== 'all' && activeLogTab !== entry.agent) div.style.display = 'none';
            document.getElementById('terminal').appendChild(div);
            document.getElementById('terminal').scrollTop = 9999;
            if (persist) workflowState.logs.push(entry);
            logAgents.add(entry.agent); renderLogTabs();
            applyStreamStatus(entry);
        }

        function applyStreamStatus(entry) {
            if (!entry.agentId) return;
            if (entry.type === 'started') {
                const old = chunkQueues.get(entry.agentId);
                if (old) { clearTimeout(old.timer); chunkQueues.delete(entry.agentId); }
                setNodeStatus(entry.agentId, 'active');
            }
            if (entry.type === 'completed') setNodeStatus(entry.agentId, 'completed');
            if (entry.type === 'awaiting_approval') setNodeStatus(entry.agentId, 'awaiting-approval');
            if (entry.type === 'approved') setNodeStatus(entry.agentId, 'completed');
            if (entry.type === 'rejected') setNodeStatus(entry.agentId, 'rejected');
            if (entry.type === 'error') setNodeStatus(entry.agentId, 'error');
        }

        function renderTerminalLogs() {
            const terminal = document.getElementById('terminal');
            terminal.innerHTML = '';
            logAgents.clear();
            activeLogTab = 'all';
            renderLogTabs();
            if (!workflowState.logs.length) appendTerminalLog('System', '16인 완전체 팀 소환 완료. 정식판 v5.0 가동.', true);
            else workflowState.logs.forEach((entry) => {
                if (entry.id) appendStreamEvent(entry, false);
                else appendTerminalLog(entry.agent, entry.message, false);
            });
        }

