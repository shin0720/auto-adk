        // PR #26b: lightweight, failure-safe local parser for pasted provider
        // responses. NO provider run, NO LLM call. Best-effort only: on any
        // failure it never throws and the caller keeps the rawText intact.
        // Tries JSON block first, then heading sections, then line prefixes.

        const PARSE_FIELDS = ['summary', 'risks', 'conflicts', 'recommendations', 'missingQuestions', 'implementationWarnings'];

        // Keyword map: header/prefix text (ko/en) -> summary field key.
        const PARSE_KEYWORDS = [
            { key: 'summary', re: /^(요약|summary|개요)/i },
            { key: 'risks', re: /^(위험|리스크|risks?)/i },
            { key: 'conflicts', re: /^(충돌|conflicts?)/i },
            { key: 'recommendations', re: /^(추천|권고|recommendations?)/i },
            { key: 'missingQuestions', re: /^(질문|추가\s*질문|missing\s*questions?|questions?)/i },
            { key: 'implementationWarnings', re: /^(경고|구현\s*경고|implementation\s*warnings?|warnings?)/i }
        ];

        function emptyParsed() {
            return { summary: '', risks: [], conflicts: [], recommendations: [], missingQuestions: [], implementationWarnings: [] };
        }

        // toList coerces a value to a string array (splits strings on newlines).
        function toList(v) {
            if (Array.isArray(v)) return v.map(x => String(x).trim()).filter(Boolean);
            if (typeof v === 'string') return v.split('\n').map(s => s.trim()).filter(Boolean);
            return [];
        }

        function tryParseJson(text) {
            // Prefer a fenced ```json block, else the first balanced {...}.
            let body = null;
            const fence = text.match(/```(?:json)?\s*([\s\S]*?)```/i);
            if (fence) body = fence[1];
            else {
                const s = text.indexOf('{'), e = text.lastIndexOf('}');
                if (s >= 0 && e > s) body = text.slice(s, e + 1);
            }
            if (!body) return null;
            try {
                const obj = JSON.parse(body);
                return obj && typeof obj === 'object' ? obj : null;
            } catch (_) { return null; }
        }

        // stripBullet removes leading list markers so items read cleanly.
        function stripBullet(line) {
            return line.replace(/^\s*(?:[-*•]|\d+[.)])\s*/, '').trim();
        }

        // matchHeader returns a field key if the line looks like a section
        // header (optionally decorated with #, *, [], :) for that keyword.
        function matchHeader(line) {
            const bare = line.replace(/^[#>\s]*/, '').replace(/[*[\]:]/g, '').trim();
            for (const kw of PARSE_KEYWORDS) {
                if (kw.re.test(bare)) {
                    // Header lines are short; a long sentence is content, not a header.
                    if (bare.length <= 24) return kw.key;
                }
            }
            return null;
        }

        // parseSections returns the parsed fields plus a `matched` flag that is
        // true only when at least one recognized header/prefix was found. Text
        // with no structure at all leaves matched=false so the caller can mark
        // it failedParse (and keep the rawText) instead of faking a summary.
        function parseSections(text) {
            const out = emptyParsed();
            const summaryLines = [];
            let cur = null, matched = false;
            for (const raw of text.split('\n')) {
                const line = raw.trim();
                if (!line) continue;
                const header = matchHeader(line);
                if (header) { cur = header; matched = true; continue; }
                // Inline "위험: foo" prefix (no dedicated header line).
                const inline = line.match(/^([^:：]{1,16})[:：]\s*(.+)$/);
                if (inline) {
                    const key = matchHeader(inline[1]);
                    if (key) { matched = true; if (key === 'summary') summaryLines.push(inline[2]); else out[key].push(inline[2].trim()); continue; }
                }
                if (cur === 'summary') summaryLines.push(line);
                else if (cur) out[cur].push(stripBullet(line));
                else summaryLines.push(line);
            }
            out.summary = summaryLines.join(' ').trim();
            out.matched = matched;
            return out;
        }

        function normalizeParsed(obj) {
            const out = emptyParsed();
            out.summary = typeof obj.summary === 'string' ? obj.summary : '';
            out.risks = toList(obj.risks);
            out.conflicts = toList(obj.conflicts);
            out.recommendations = toList(obj.recommendations);
            out.missingQuestions = toList(obj.missingQuestions || obj.questions);
            out.implementationWarnings = toList(obj.implementationWarnings || obj.warnings);
            return out;
        }

        function parsedHasContent(p) {
            return !!(p.summary || p.risks.length || p.conflicts.length || p.recommendations.length || p.missingQuestions.length || p.implementationWarnings.length);
        }

        // parseProviderResponse never throws. parseStatus is 'parsed' when any
        // structure was recovered, else 'failedParse' (rawText stays with the
        // caller regardless).
        function parseProviderResponse(rawText) {
            try {
                const text = (rawText || '').trim();
                if (!text) return { parseStatus: 'failedParse', ...emptyParsed() };
                const json = tryParseJson(text);
                if (json) {
                    const p = normalizeParsed(json);
                    if (parsedHasContent(p)) return { parseStatus: 'parsed', ...p };
                }
                const sect = parseSections(text);
                if (sect.matched && parsedHasContent(sect)) {
                    delete sect.matched;
                    return { parseStatus: 'parsed', ...sect };
                }
                return { parseStatus: 'failedParse', ...emptyParsed() };
            } catch (_) {
                return { parseStatus: 'failedParse', ...emptyParsed() };
            }
        }
