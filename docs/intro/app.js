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
        `Download <a href="${ASSET('local-eml-windows-amd64.exe')}"><code>local-eml-windows-amd64.exe</code></a> and double-click it. The first time, Windows may show "Windows protected your PC" because the file is new to it: choose <strong>More info → Run anyway</strong>.`,
      'install.mac':
        `Download <a href="${ASSET('local-eml-darwin-arm64.app.zip')}"><code>local-eml-darwin-arm64.app.zip</code></a> (Apple silicon) or <a href="${ASSET('local-eml-darwin-amd64.app.zip')}"><code>local-eml-darwin-amd64.app.zip</code></a> (Intel), unzip it and double-click <strong>Local Eml</strong>. If macOS says it can't verify the developer, go to <strong>System Settings → Privacy &amp; Security</strong> and choose <strong>Open Anyway</strong>, then open it again.`,
      'install.linux': `Download <a href="${ASSET('local-eml-linux-amd64')}"><code>local-eml-linux-amd64</code></a>, mark it executable (<code>chmod +x</code>), and run it. It opens in your browser.`,
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
        `<a href="${ASSET('local-eml-windows-amd64.exe')}"><code>local-eml-windows-amd64.exe</code></a>를 내려받아 더블클릭하세요. 처음 한 번은 아직 알려지지 않은 파일이라 "Windows의 PC 보호" 창이 뜰 수 있습니다. <strong>추가 정보 → 실행</strong>을 선택하세요.`,
      'install.mac':
        `<a href="${ASSET('local-eml-darwin-arm64.app.zip')}"><code>local-eml-darwin-arm64.app.zip</code></a>(Apple silicon) 또는 <a href="${ASSET('local-eml-darwin-amd64.app.zip')}"><code>local-eml-darwin-amd64.app.zip</code></a>(Intel)를 내려받아 압축을 풀고 <strong>Local Eml</strong>을 더블클릭하세요. 개발자를 확인할 수 없다고 나오면 <strong>시스템 설정 → 개인정보 보호 및 보안</strong>에서 <strong>그래도 열기</strong>를 누른 뒤 다시 여세요.`,
      'install.linux': `<a href="${ASSET('local-eml-linux-amd64')}"><code>local-eml-linux-amd64</code></a>를 내려받아 실행 권한을 주고(<code>chmod +x</code>) 실행하세요. 브라우저에서 열립니다.`,
      'install.howto':
        '따라하기: <a href="docs/ko/how-to/open-eml-files">.eml 파일 열기</a> · <a href="docs/ko/how-to/takeout-mbox">Google 테이크아웃 .mbox 가져오기</a> · <a href="docs/ko/how-to/pst-without-outlook">Outlook 없이 .pst 열기</a> · <a href="docs/ko/how-to/free-up-gmail">Gmail 용량 확보하기</a>',
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
    ja: {
      'nav.screens': '画面',
      'nav.install': 'ダウンロード',
      'nav.features': '機能',
      'nav.docs': 'ドキュメント',
      'hero.eyebrow': '無料・オープンソースのEMLビューア',
      'hero.title': '<span class="line">古いメールを開いて、</span><span class="line">自分のPCに保管する。</span>',
      'hero.lead':
        'Windows・macOS・Linuxで<code>.eml</code>、<code>.mbox</code>、<code>.pst</code>ファイルを開いて保管できるビューアです。いっぱいになったメールボックスからメールを取り出し、検索して、いつでも読めます。実行ファイルは1つだけ。アカウントは不要で、データがPCの外に出ることはありません。',
      'hero.download': 'ダウンロード',
      'hero.github': 'GitHubで見る',
      'hero.other': 'ほかのプラットフォーム',
      'hero.badges': 'Windows · macOS · Linux · GPL-3.0',
      'screens.title': '落ち着いて読める場所',
      'screens.sub': 'すべて、普段お使いのブラウザの中で127.0.0.1上で動きます。見た目はサーバーではなく、アプリです。',
      'screens.library': 'ライブラリ',
      'screens.search': '検索',
      'screens.viewer': '閲覧',
      'screens.stats': '統計',
      'screens.import': 'インポート',
      'screens.cap.library': 'スレッドごとのまとめ表示、スター、色分けカテゴリ、期間の指定。',
      'screens.cap.search': '一致した箇所をハイライトする全文検索。ハングルの初声検索（초성검색）もライブラリ全体で使えます。',
      'screens.cap.viewer': 'HTMLメールはサンドボックス化したフレームに表示し、外部画像はブロック。添付ファイルの一覧も、スレッド全体もワンクリックで開けます。',
      'screens.cap.stats': 'どれくらいのメールが、誰から、いつ届いたのか。何を残すか決めるときに役立ちます。',
      'screens.cap.import': 'ファイル、フォルダ、アーカイブをドロップするか、S3やIMAPから取り込みます。進行状況はリアルタイムで表示され、重複はスキップされます。',
      'install.title': 'ダウンロードしてダブルクリック',
      'install.sub':
        'インストーラーもアカウントも不要です。お使いのOS用のファイルをダウンロードして開くと、Local Emlが専用のウィンドウで起動します。使い終わってウィンドウを閉じれば、裏で動き続けるものはありません。',
      'install.win':
        `<a href="${ASSET('local-eml-windows-amd64.exe')}"><code>local-eml-windows-amd64.exe</code></a>をダウンロードしてダブルクリックします。初回は、まだ広く知られていないファイルのため「Windows によって PC が保護されました」と表示されることがあります。<strong>詳細情報 → 実行</strong>を選んでください。`,
      'install.mac':
        `<a href="${ASSET('local-eml-darwin-arm64.app.zip')}"><code>local-eml-darwin-arm64.app.zip</code></a>（Appleシリコン）または<a href="${ASSET('local-eml-darwin-amd64.app.zip')}"><code>local-eml-darwin-amd64.app.zip</code></a>（Intel）をダウンロードして展開し、<strong>Local Eml</strong>をダブルクリックします。開発元を確認できないと表示されたら、<strong>システム設定 → プライバシーとセキュリティ</strong>で<strong>このまま開く</strong>を選び、もう一度開いてください。`,
      'install.linux': `<a href="${ASSET('local-eml-linux-amd64')}"><code>local-eml-linux-amd64</code></a>をダウンロードし、実行権限を付けて（<code>chmod +x</code>）実行します。ブラウザで開きます。`,
      'install.howto':
        '手順ガイド：<a href="docs/ja/how-to/open-eml-files">.emlファイルを開く</a> · <a href="docs/ja/how-to/takeout-mbox">Google データエクスポートの.mboxを取り込む</a> · <a href="docs/ja/how-to/pst-without-outlook">Outlookなしで.pstを開く</a> · <a href="docs/ja/how-to/free-up-gmail">Gmailの容量を空ける</a>',
      'install.svc.title': '常に起動しておきたい場合',
      'install.svc.sub':
        '1行のコマンドで、起動時に自動で立ち上がり、新着のIMAPメールを受信し続けるバックグラウンドサービスとしてインストールできます。最新リリースをダウンロードし、先に検証を行います。',
      'install.copy': 'コピー',
      'install.copied': 'コピーしました',
      'install.winget.title': 'Windowsでwingetを使う場合：',
      'install.winget.body': '<code>winget install hwhang0917.LocalEml</code>（登録審査中）',
      'features.title': 'できること',
      'f.import.t': '何でもインポート',
      'f.import.b':
        '<code>.eml</code>ファイル、フォルダ、<code>.zip</code>、<code>.mbox</code>（Google データエクスポート、Thunderbird）、Outlookの<code>.pst</code>、S3バケット、IMAPメールボックス。重複はハッシュで判定してスキップします。',
      'f.search.t': '日本語もそのまま検索',
      'f.search.b':
        '差出人、件名、本文をまとめて全文検索します。日本語・韓国語・中国語は設定なしで検索でき、ハングルは初声だけの入力（<code>ㅎㄱ</code> → 한국）でも見つかります。',
      'f.private.t': '設計からプライベート',
      'f.private.b':
        '待ち受けるのは<code>127.0.0.1</code>だけ。アカウントも、テレメトリも、クラウドもありません。IMAPパスワードは自分でオンにしない限り保存されず、保存する場合もディスクから出ない鍵で暗号化されます。',
      'f.safe.t': 'HTMLも安全に表示',
      'f.safe.b': 'メールはサンドボックス化したフレームで表示され、外部画像は既定でブロックされるため、トラッキングピクセルは機能しません。',
      'f.sync.t': '新着メールにも対応',
      'f.sync.b':
        'IMAPプロファイルでバックグラウンド同期をオンにすると、前回以降に届いたメールだけを取得します。読み取り専用なので、メールボックスが変更されることはありません。',
      'f.export.t': 'いつでも持ち出せる',
      'f.export.b': 'ライブラリ全体を1つの<code>.zip</code>としてエクスポートするか、S3に送れます。スター、カテゴリ、設定も一緒です。',
      'not.t': 'できないこと：',
      'not.b':
        'メールクライアントではありません。作成、送信、返信はできません。日々のメールはいつものクライアントで、手元に残しておきたいメールはLocal Emlで。',
      'foot.contrib': 'コントリビュート',
      'foot.releases': 'リリース',
      os: { windows: 'Windows版', mac: 'macOS版', linux: 'Linux版' },
      title: 'Local Eml — 無料・オープンソースのEMLビューア（Windows・macOS・Linux）',
    },
  }

  const LANGS = Object.keys(I18N)
  // Locales without their own screenshots borrow another locale's.
  const SHOT_LANG = { ja: 'en' }

  const $ = (s, r = document) => r.querySelector(s)
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s))
  const prefersDark = matchMedia('(prefers-color-scheme: dark)')

  function detectLang() {
    const q = new URLSearchParams(location.search).get('lang')
    if (LANGS.includes(q)) return q
    try {
      const s = localStorage.getItem('intro-lang')
      if (LANGS.includes(s)) return s
    } catch {}
    const browser = navigator.language?.toLowerCase().split('-')[0]
    return LANGS.includes(browser) ? browser : 'en'
  }

  let lang = detectLang()
  let shot = 'viewer' // the hero already shows the library

  function shotSrc(name, dark) {
    return `img/${SHOT_LANG[lang] || lang}${dark ? '-dark' : ''}/${name}.webp`
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
    if (/Mac OS X|Macintosh/i.test(ua)) return { key: 'mac', asset: 'local-eml-darwin-arm64.app.zip' }
    if (/Linux/i.test(ua)) return { key: 'linux', asset: 'local-eml-linux-amd64' }
    return { key: '', asset: '' }
  }

  $$('.lang button').forEach((b) =>
    b.addEventListener('click', () => {
      lang = b.dataset.lang
      try {
        localStorage.setItem('intro-lang', lang)
      } catch {}
      history.replaceState(null, '', lang === 'en' ? location.pathname : `?lang=${lang}`)
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
