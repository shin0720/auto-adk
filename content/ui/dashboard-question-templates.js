        // PR #25: deterministic question templates (no provider call).
        // Each question: { id, group, category, text, required, type, round, source }.
        // group: 'intake' | 'readiness' | 'uiux'. type: 'textarea'|'text'.
        // round applies to uiux design discovery (1=취향, 2=레퍼런스, 3=재질문).

        const INTAKE_QUESTION_TEMPLATES = [
            { id: 'in-purpose',   group: 'intake', category: '목적',        text: '이 제품이 해결하려는 핵심 문제를 한 문장으로 설명해 주세요.', required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'in-users',     group: 'intake', category: '대상 사용자',  text: '주 사용자는 누구이며 기술 수준은 어느 정도인가요?',       required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'in-core',      group: 'intake', category: '핵심 기능',    text: '반드시 있어야 할 핵심 기능 3가지는 무엇인가요?',          required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'in-exclude',   group: 'intake', category: '제외 기능',    text: '이번 범위에서 명시적으로 빼는 기능이 있나요?',            required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'in-input',     group: 'intake', category: '입력 데이터',  text: '시스템에 들어오는 데이터의 형식과 출처는 무엇인가요?',    required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'in-output',    group: 'intake', category: '출력 형식',    text: '결과는 어떤 형태(화면/파일/API)로 나와야 하나요?',        required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'in-platform',  group: 'intake', category: '플랫폼/환경',  text: '웹/모바일/데스크톱/CLI 중 무엇이고 실행 환경은 무엇인가요?', required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'in-automation',group: 'intake', category: '자동화 수준',  text: 'manual / supervised / auto 중 어디까지 자동화를 원하나요?', required: true,  type: 'text',     round: 0, source: 'template' },
            { id: 'in-success',   group: 'intake', category: '성공 기준',    text: '무엇이 되면 "완성"인가요? 측정 가능한 값으로 알려주세요.', required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'in-priority',  group: 'intake', category: '우선순위/마감', text: '속도/품질/비용 중 우선순위와 이번 반복의 마감은 언제인가요?', required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'in-forbidden', group: 'intake', category: '금지사항',     text: '절대 하지 말아야 할 것이 있나요?',                        required: false, type: 'textarea', round: 0, source: 'template' }
        ];

        const READINESS_QUESTION_TEMPLATES = [
            { id: 'rd-install', group: 'readiness', category: '설치/환경',      text: '로컬 OS와 이미 설치된 런타임(Node/Go/Python) 버전은 무엇인가요?', required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-api',     group: 'readiness', category: 'API/계정/비밀키', text: '연동할 외부 서비스와 API 키/계정을 직접 준비할 수 있나요?',       required: true,  type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-data',    group: 'readiness', category: '데이터 준비',    text: '초기 데이터(이미지/CSV/문서)를 제공하나요, 생성해야 하나요?',    required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-deploy',  group: 'readiness', category: '배포/운영',     text: '어디에 배포하나요(Vercel/자체서버/데스크톱)? 계정이 있나요?',     required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-cost',    group: 'readiness', category: '비용/요금제',   text: '유료 API/호스팅 비용 상한이 있나요?',                            required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-legal',   group: 'readiness', category: '법적/라이선스', text: '상용 사용인가요? 라이선스 제약이 있는 자산을 쓰나요?',           required: false, type: 'textarea', round: 0, source: 'template' },
            { id: 'rd-fallback',group: 'readiness', category: '실패 fallback', text: '외부 연동이 불가할 때 STUB로 진행해도 되나요?',                   required: true,  type: 'text',     round: 0, source: 'template' }
        ];

        const UIUX_QUESTION_TEMPLATES = [
            { id: 'ux-mood',     group: 'uiux', category: '전체 분위기',      text: '차분/생동감/고급/친근 중 어디에 가까운가요?',              required: true,  type: 'text',     round: 1, source: 'template' },
            { id: 'ux-ref-app',  group: 'uiux', category: '참고 서비스/앱',   text: '닮고 싶은 앱이나 사이트 2~3개를 알려주세요.',              required: false, type: 'textarea', round: 1, source: 'template' },
            { id: 'ux-ref-img',  group: 'uiux', category: '참고 이미지',      text: '무드보드나 스크린샷을 참고 자료로 제공할 수 있나요?',      required: false, type: 'textarea', round: 2, source: 'template' },
            { id: 'ux-color',    group: 'uiux', category: '색감',            text: '선호하거나 피해야 할 색이 있나요? 다크/라이트 중 무엇인가요?', required: false, type: 'textarea', round: 1, source: 'template' },
            { id: 'ux-type',     group: 'uiux', category: '타이포그래피',    text: '또렷한 산세리프와 개성 있는 서체 중 어느 쪽을 선호하나요?', required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-density',  group: 'uiux', category: '여백/정보 밀도',  text: '여백이 넉넉한 화면과 정보가 조밀한 화면 중 무엇을 원하나요?', required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-comp',     group: 'uiux', category: '카드/버튼/패널',  text: '둥근/각진 모서리, 그림자 강도는 어느 정도가 좋나요?',      required: false, type: 'textarea', round: 1, source: 'template' },
            { id: 'ux-icon',     group: 'uiux', category: '아이콘/일러스트', text: '아이콘만 쓸지, 일러스트나 캐릭터를 포함할지 알려주세요.',  required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-device',   group: 'uiux', category: '모바일/데스크톱', text: '우선 타깃 화면은 모바일인가요, 데스크톱인가요?',            required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-brand',    group: 'uiux', category: '브랜드/로고',     text: '기존 로고나 브랜드 컬러가 있나요?',                        required: false, type: 'textarea', round: 1, source: 'template' },
            { id: 'ux-hate',     group: 'uiux', category: '절대 싫은 스타일', text: '피하고 싶은 디자인 스타일이 있나요?',                     required: true,  type: 'textarea', round: 1, source: 'template' },
            { id: 'ux-motion',   group: 'uiux', category: '애니메이션',      text: '정적인 화면과 부드러운 인터랙션 중 무엇을 선호하나요?',     required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-a11y',     group: 'uiux', category: '접근성/가독성',   text: '고대비나 큰 글씨 같은 접근성 요구가 있나요?',              required: false, type: 'text',     round: 1, source: 'template' },
            { id: 'ux-polish',   group: 'uiux', category: '배포급 polish',   text: '내부 프로토타입 수준인가요, 배포급 완성도가 필요한가요?',   required: true,  type: 'text',     round: 1, source: 'template' }
        ];

        // clarityTier inspects the seed request text and returns how many questions
        // to surface. Keyword-driven, fully deterministic — no provider call.
        function clarityTier(requestText) {
            const t = (requestText || '').trim();
            const commercial = /배포|상용|결제|판매|출시|런칭|launch|deploy|payment|자동화|auto/i.test(t);
            if (commercial) return { tier: 'commercial', min: 20, max: 30 };
            if (t.length === 0) return { tier: 'vague', min: 20, max: 30 };
            if (t.length < 40) return { tier: 'vague', min: 20, max: 30 };
            if (t.length < 120) return { tier: 'normal', min: 15, max: 20 };
            return { tier: 'clear', min: 8, max: 12 };
        }

        const ALL_QUESTION_TEMPLATES = INTAKE_QUESTION_TEMPLATES
            .concat(READINESS_QUESTION_TEMPLATES)
            .concat(UIUX_QUESTION_TEMPLATES);
