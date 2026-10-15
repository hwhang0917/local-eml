/* Static landing page: language switch, screenshot tabs, OS-aware download, copy buttons. */
;(() => {
  const REPO = 'https://github.com/hwhang0917/local-eml'
  const ASSET = (name) => `${REPO}/releases/latest/download/${name}`

  const I18N = {
    en: {
      'nav.screens': 'Screens',
      'nav.install': 'Download',
      'nav.features': 'Features',
      'nav.docs': 'Docs',
      'hero.eyebrow': 'Free, open-source EML viewer',
      'hero.title': '<span class="line">Open your old mail.</span><span class="line">Keep it on your own disk.</span>',
      'hero.lead':
        'A viewer and archive for <code>.eml</code>, <code>.mbox</code> and <code>.pst</code> files on Windows, macOS and Linux. Pull messages out of an overflowing mailbox, search them, and read them any time. One file to run, no account, nothing leaves your machine.',
      'hero.download': 'Download',
      'hero.github': 'View on GitHub',
      'hero.other': 'Other platforms',
      'hero.badges': 'Windows · macOS · Linux · GPL-3.0',
      'screens.title': 'A quiet place to read',
      'screens.sub': 'Everything runs on 127.0.0.1 in your normal browser. It looks like an app, not a server.',
      'screens.library': 'Library',
      'screens.search': 'Search',
      'screens.viewer': 'Reading',
      'screens.stats': 'Stats',
      'screens.import': 'Import',
      'screens.cap.library': 'Threads grouped by conversation, stars, colour categories, and a date range picker.',
      'screens.cap.search': 'Full-text search with highlighted matches. Korean initial-consonant search (초성검색) works across the whole library.',
      'screens.cap.viewer': 'HTML mail in a sandboxed frame, remote images blocked, attachments listed, the whole conversation one click away.',
      'screens.cap.stats': 'How much mail, from whom, and when. Handy for deciding what to keep.',
      'screens.cap.import': 'Drop files, folders or archives, or pull from S3 and IMAP. Progress streams live and duplicates are skipped.',
      'install.title': 'Download and double-click',
      'install.sub':
        "No installer, no account. Download the file for your OS and open it; Local Eml appears in its own window. Close the window when you're done and nothing keeps running.",
      'install.win':
        'Download <code>local-eml-windows-amd64.exe</code> and double-click it. The first time, Windows may show "Windows protected your PC" because the file is new to it: choose <strong>More info → Run anyway</strong>.',
      'install.mac':
        "Download <code>local-eml-darwin-arm64</code> (Apple silicon) or <code>-amd64</code> (Intel) and open it. If macOS says it can't verify the developer, go to <strong>System Settings → Privacy &amp; Security</strong> and choose <strong>Open Anyway</strong>, then open it again.",
      'install.linux': 'Download <code>local-eml-linux-amd64</code>, mark it executable (<code>chmod +x</code>), and run it. It opens in your browser.',
      'install.howto':
        'Step-by-step guides: <a href="docs/how-to/open-eml-files">open .eml files</a> · <a href="docs/how-to/takeout-mbox">import a Google Takeout .mbox</a> · <a href="docs/how-to/pst-without-outlook">open a .pst without Outlook</a> · <a href="docs/how-to/free-up-gmail">free up Gmail storage</a>',
      'install.svc.title': 'Want it running all the time?',
      'install.svc.sub':
        'One line installs it as a background service that starts on boot and keeps fetching new IMAP mail. It downloads the latest release and verifies it first.',
      'install.copy': 'Copy',
      'install.copied': 'Copied',
      'install.winget.title': 'Windows with winget:',
      'install.winget.body': '<code>winget install hwhang0917.LocalEml</code> (listing pending review).',
      'features.title': 'What it does',
      'f.import.t': 'Import anything',
      'f.import.b':
        '<code>.eml</code> files, folders, <code>.zip</code>, <code>.mbox</code> (Google Takeout, Thunderbird), Outlook <code>.pst</code>, an S3 bucket, or an IMAP mailbox. Duplicates are skipped by hash.',
      'f.search.t': 'Search that speaks CJK',
      'f.search.b':
        'Full-text search over sender, subject and body. Korean, Japanese and Chinese work out of the box, and typing just the initial consonants (<code>ㅎㄱ</code> → 한국) finds it too.',
      'f.private.t': 'Private by design',
      'f.private.b':
        'Listens only on <code>127.0.0.1</code>. No account, no telemetry, no cloud. IMAP passwords are never stored unless you opt in, and then only encrypted with a key that stays on your disk.',
      'f.safe.t': 'Reads HTML safely',
      'f.safe.b': 'Messages render in a sandboxed frame with remote images blocked by default, so tracking pixels stay blind.',
      'f.sync.t': 'Keeps up with new mail',
      'f.sync.b':
        'Turn on background sync for any IMAP profile and it fetches only what is new since last time. Read-only: your mailbox is never modified.',
      'f.export.t': 'Yours to take with you',
      'f.export.b': 'Export the whole library as a single <code>.zip</code> or push it to S3. Stars, categories and settings travel with it.',
      'not.t': "What it isn't:",
      'not.b':
        "a mail client. You can't compose, send or reply. Keep your usual client for daily mail; use Local Eml for the mail you want to keep but not carry.",
      'foot.contrib': 'Contributing',
      'foot.releases': 'Releases',
      os: { windows: 'for Windows', mac: 'for macOS', linux: 'for Linux' },
      title: 'Local Eml — free, open-source EML viewer for Windows, macOS and Linux',
    },
    ko: {
      'nav.screens': '화면',
      'nav.install': '다운로드',
      'nav.features': '기능',
      'nav.docs': '문서',
      'hero.eyebrow': '무료 오픈소스 EML 뷰어',
      'hero.title': '<span class="line">오래된 메일을 열어 보고,</span><span class="line">내 PC에 그대로 보관하세요.</span>',
      'hero.lead':
        'Windows·macOS·Linux에서 <code>.eml</code>, <code>.mbox</code>, <code>.pst</code> 파일을 열어 보고 보관하는 뷰어입니다. 가득 찬 메일함에서 메일을 꺼내 검색하고 언제든 읽으세요. 실행 파일 하나, 계정 없음, 데이터는 내 PC를 벗어나지 않습니다.',
      'hero.download': '다운로드',
      'hero.github': 'GitHub에서 보기',
      'hero.other': '다른 플랫폼',
      'hero.badges': 'Windows · macOS · Linux · GPL-3.0',
      'screens.title': '조용히 읽을 수 있는 곳',
      'screens.sub': '평소 쓰는 브라우저에서 127.0.0.1로만 동작합니다. 서버가 아니라 앱처럼 느껴지도록 만들었습니다.',
      'screens.library': '라이브러리',
      'screens.search': '검색',
      'screens.viewer': '읽기',
      'screens.stats': '통계',
      'screens.import': '가져오기',
      'screens.cap.library': '대화별 묶기, 별표, 색상 분류, 날짜 범위 선택까지 한 화면에.',
      'screens.cap.search': '검색어가 강조된 전문 검색. 초성만 입력하는 초성검색도 라이브러리 전체에서 동작합니다.',
      'screens.cap.viewer': 'HTML 메일은 격리된 프레임에서, 외부 이미지는 차단된 채로. 첨부 파일 목록과 대화 전체가 한 번의 클릭 거리에 있습니다.',
      'screens.cap.stats': '얼마나, 누구에게서, 언제 받았는지. 무엇을 남길지 정할 때 유용합니다.',
      'screens.cap.import': '파일·폴더·압축 파일을 드롭하거나 S3와 IMAP에서 바로 가져옵니다. 진행 상황은 실시간으로, 중복은 자동으로 건너뜁니다.',
      'install.title': '내려받아 더블클릭',
      'install.sub': '설치 프로그램도 계정도 없습니다. 내 운영체제에 맞는 파일을 내려받아 열면 Local Eml이 전용 창으로 나타납니다. 다 보고 창을 닫으면 그걸로 끝, 뒤에서 돌아가는 건 없습니다.',
      'install.win':
        '<code>local-eml-windows-amd64.exe</code>를 내려받아 더블클릭하세요. 처음 한 번은 아직 알려지지 않은 파일이라 "Windows의 PC 보호" 창이 뜰 수 있습니다. <strong>추가 정보 → 실행</strong>을 선택하세요.',
      'install.mac':
        '<code>local-eml-darwin-arm64</code>(Apple silicon) 또는 <code>-amd64</code>(Intel)를 내려받아 여세요. 개발자를 확인할 수 없다고 나오면 <strong>시스템 설정 → 개인정보 보호 및 보안</strong>에서 <strong>그래도 열기</strong>를 누른 뒤 다시 여세요.',
      'install.linux': '<code>local-eml-linux-amd64</code>를 내려받아 실행 권한을 주고(<code>chmod +x</code>) 실행하세요. 브라우저에서 열립니다.',
      'install.howto':
        '따라하기: <a href="docs/ko/how-to/open-eml-files">.eml 파일 열기</a> · <a href="docs/ko/how-to/takeout-mbox">Google 테이크아웃 .mbox 가져오기</a> · <a href="docs/ko/how-to/pst-without-outlook">Outlook 없이 .pst 열기</a> · <a href="docs/ko/how-to/free-up-gmail">Gmail 용량 비우기</a>',
      'install.svc.title': '항상 켜 두고 싶다면',
      'install.svc.sub': '한 줄이면 부팅 시 자동으로 시작되고 새 IMAP 메일을 계속 받아오는 백그라운드 서비스로 설치됩니다. 최신 릴리스를 내려받아 무결성을 먼저 확인합니다.',
      'install.copy': '복사',
      'install.copied': '복사됨',
      'install.winget.title': 'Windows winget:',
      'install.winget.body': '<code>winget install hwhang0917.LocalEml</code> (등록 심사 중)',
      'features.title': '이런 걸 합니다',
      'f.import.t': '무엇이든 가져오기',
      'f.import.b':
        '<code>.eml</code> 파일, 폴더, <code>.zip</code>, <code>.mbox</code>(Google 테이크아웃, Thunderbird), Outlook <code>.pst</code>, S3 버킷, IMAP 메일함. 중복은 해시로 걸러냅니다.',
      'f.search.t': '한글이 잘 되는 검색',
      'f.search.b':
        '보낸 사람, 제목, 본문을 한 번에 전문 검색합니다. 한국어·일본어·중국어가 그대로 검색되고, 초성만 입력해도(<code>ㅎㄱ</code> → 한국) 찾아냅니다.',
      'f.private.t': '설계부터 프라이빗',
      'f.private.b':
        '<code>127.0.0.1</code>에서만 듣습니다. 계정도, 텔레메트리도, 클라우드도 없습니다. IMAP 비밀번호는 직접 켜지 않는 한 저장하지 않고, 켜더라도 내 디스크에만 있는 키로 암호화합니다.',
      'f.safe.t': 'HTML도 안전하게',
      'f.safe.b': '메일은 격리된 프레임에서 렌더링되고 외부 이미지는 기본 차단이라 추적 픽셀이 동작하지 않습니다.',
      'f.sync.t': '새 메일도 놓치지 않게',
      'f.sync.b': 'IMAP 프로필에 백그라운드 동기화를 켜 두면 마지막 이후 도착한 메일만 받아옵니다. 읽기 전용이라 메일함은 절대 건드리지 않습니다.',
      'f.export.t': '언제든 들고 나갈 수 있게',
      'f.export.b': '라이브러리 전체를 <code>.zip</code> 하나로 내려받거나 S3에 올립니다. 별표·분류·설정도 함께 갑니다.',
      'not.t': '이런 건 아닙니다:',
      'not.b': '메일 클라이언트가 아닙니다. 쓰기·보내기·답장은 할 수 없어요. 평소 메일은 쓰던 클라이언트에서, 오래 남길 메일만 Local Eml에 보관하세요.',
      'foot.contrib': '기여하기',
      'foot.releases': '릴리스',
      os: { windows: 'Windows용', mac: 'macOS용', linux: 'Linux용' },
      title: 'Local Eml — 무료 오픈소스 EML 뷰어 (Windows·macOS·Linux)',
    },
  }

  const $ = (s, r = document) => r.querySelector(s)
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s))
  const prefersDark = matchMedia('(prefers-color-scheme: dark)')

  function detectLang() {
    const q = new URLSearchParams(location.search).get('lang')
    if (q === 'en' || q === 'ko') return q
    try {
      const s = localStorage.getItem('intro-lang')
      if (s === 'en' || s === 'ko') return s
    } catch {}
    return navigator.language?.toLowerCase().startsWith('ko') ? 'ko' : 'en'
  }

  let lang = detectLang()
  let shot = 'viewer' // the hero already shows the library

  function shotSrc(name, dark) {
    return `img/${lang}${dark ? '-dark' : ''}/${name}.webp`
  }

  function render() {
    const t = I18N[lang]
    document.documentElement.lang = lang
    document.title = t.title
    $$('[data-i18n]').forEach((el) => {
      const v = t[el.dataset.i18n]
      if (v !== undefined) el.innerHTML = v
    })
    $$('.lang button').forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.lang === lang)))
    $('#hero-img').src = shotSrc('library', prefersDark.matches)
    $('#hero-img').alt = t['screens.cap.library'].replace(/<[^>]+>/g, '')
    $('#dl-os').textContent = t.os[detectOS().key] || ''
    renderShot()
  }

  function renderShot() {
    const t = I18N[lang]
    $$('#shot-tabs [role=tab]').forEach((b) => b.setAttribute('aria-selected', String(b.dataset.shot === shot)))
    $('#shot-img').src = shotSrc(shot, false)
    $('#shot-caption').innerHTML = t[`screens.cap.${shot}`]
    $('#shot-img').alt = t[`screens.cap.${shot}`].replace(/<[^>]+>/g, '')
  }

  function detectOS() {
    const ua = navigator.userAgent
    if (/Windows/i.test(ua)) return { key: 'windows', asset: 'local-eml-windows-amd64.exe' }
    if (/Mac OS X|Macintosh/i.test(ua)) return { key: 'mac', asset: 'local-eml-darwin-arm64' }
    if (/Linux/i.test(ua)) return { key: 'linux', asset: 'local-eml-linux-amd64' }
    return { key: '', asset: '' }
  }

  $$('.lang button').forEach((b) =>
    b.addEventListener('click', () => {
      lang = b.dataset.lang
      try {
        localStorage.setItem('intro-lang', lang)
      } catch {}
      history.replaceState(null, '', lang === 'ko' ? '?lang=ko' : location.pathname)
      render()
    }),
  )
  $$('#shot-tabs [role=tab]').forEach((b) =>
    b.addEventListener('click', () => {
      shot = b.dataset.shot
      renderShot()
    }),
  )
  $$('.copy').forEach((b) =>
    b.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(b.dataset.copy)
        b.textContent = I18N[lang]['install.copied']
        setTimeout(() => (b.textContent = I18N[lang]['install.copy']), 1500)
      } catch {}
    }),
  )
  prefersDark.addEventListener('change', render)

  const os = detectOS()
  if (os.asset) $('#dl-primary').href = ASSET(os.asset)
  $('#year').textContent = String(new Date().getFullYear())
  render()
})()
