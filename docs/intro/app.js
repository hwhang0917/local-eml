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
      'screens.attachments': 'Attachments',
      'screens.cap.attachments': 'Every file from every message in one table. Filter by type or sender, sort by size, and open the email it came in.',
      'screens.cap.library': 'Threads grouped by conversation, stars, colour categories and read tracking. Select several rows to mark or delete them together.',
      'screens.cap.search': 'Full-text search with highlighted matches and filters like <code>from:</code>, <code>has:attachment</code> and <code>after:</code>. Save a search for later, or export its results as a .zip, an .mbox or a PDF.',
      'screens.cap.viewer': 'HTML mail in a sandboxed frame with remote images blocked, the whole conversation one click away, and buttons to print or delete.',
      'screens.cap.stats': 'Mail over time, when it arrives, who sends the most and what takes up space. Handy for deciding what to keep.',
      'screens.cap.import': 'Drop files, folders or archives, or pull from S3 and IMAP. Saved accounts sit on cards, with a switch for background sync.',
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
        '<code>.eml</code> files, folders, <code>.zip</code>, <code>.mbox</code> (Google Takeout, Thunderbird), Outlook <code>.pst</code>, an IMAP mailbox, or an S3 bucket on AWS, MinIO, R2 or B2. Duplicates are skipped by hash.',
      'f.search.t': 'Search that speaks CJK',
      'f.search.b':
        'Full-text search over sender, subject, body and attachment names, with filters like <code>from:</code> and <code>has:attachment</code> that you can save. Korean, Japanese and Chinese work out of the box, and typing just the initial consonants (<code>ㅎㄱ</code> → 한국) finds it too.',
      'f.private.t': 'Private by design',
      'f.private.b':
        'Listens only on <code>127.0.0.1</code>. No account, no telemetry, no cloud. IMAP passwords are never stored unless you opt in, and then only encrypted, with the key kept in your system\'s keychain where there is one.',
      'f.safe.t': 'Reads HTML safely',
      'f.safe.b': 'Messages render in a sandboxed frame with remote images blocked by default, so tracking pixels stay blind.',
      'f.sync.t': 'Keeps up with new mail',
      'f.sync.b':
        'Turn on background sync for an IMAP profile and new mail lands within seconds on servers with IMAP IDLE, with an optional desktop notification. Read-only: your mailbox is never modified.',
      'f.export.t': 'Yours to take with you',
      'f.export.b': 'Export the whole library as a single <code>.zip</code> or push it to S3, or export just a search\'s results. Automatic backups copy new mail into a folder every day or week.',
      'f.attach.t': 'Every attachment in one place',
      'f.attach.b': 'One table of every file in every message. Filter by name, type, sender or date, sort by size, and preview Hangul (HWP), Word and Excel files right in the app.',
      'f.stats.t': 'See what you have',
      'f.stats.b': 'Charts of your mail over time, a weekday-by-hour grid of when it arrives, top senders and domains, and what takes up space.',
      'f.tidy.t': 'Tidy up and print',
      'f.tidy.b': 'Select many messages to mark them read or unread or delete them. Print a message or save it as a PDF, and merge mail you imported twice.',
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
      'screens.sub': '평소 쓰는 브라우저에서 열리지만, 모든 것이 내 컴퓨터(127.0.0.1) 안에서만 돌아갑니다. 서버가 아니라 앱처럼 쓸 수 있게 만들었습니다.',
      'screens.library': '라이브러리',
      'screens.search': '검색',
      'screens.viewer': '읽기',
      'screens.stats': '통계',
      'screens.import': '가져오기',
      'screens.attachments': '첨부파일',
      'screens.cap.attachments': '모든 메일의 첨부파일을 한 표에. 종류나 보낸 사람으로 거르고, 크기순으로 정렬하고, 붙어 온 메일을 바로 엽니다.',
      'screens.cap.library': '대화별 묶기, 별표, 색상 분류, 읽음 표시까지 한 화면에. 여러 줄을 골라 한꺼번에 표시하거나 지울 수 있습니다.',
      'screens.cap.search': '본문까지 찾고, 찾은 단어는 강조해서 보여 줍니다. <code>from:</code>, <code>has:attachment</code>, <code>after:</code> 같은 필터도 쓸 수 있고, 초성만 입력해도 찾아냅니다. 검색은 저장해 두거나 결과만 .zip, .mbox, PDF로 내보낼 수 있습니다.',
      'screens.cap.viewer': 'HTML 메일은 격리된 영역에서, 외부 이미지는 막힌 채로. 대화 전체가 한 번의 클릭 거리에 있고, 인쇄와 삭제도 바로 할 수 있습니다.',
      'screens.cap.stats': '기간별 메일 수, 메일이 오는 시간대, 많이 보낸 사람, 용량을 차지하는 것까지. 무엇을 남길지 정할 때 유용합니다.',
      'screens.cap.import': '파일·폴더·압축 파일을 끌어다 놓거나 S3와 IMAP에서 바로 가져옵니다. 저장한 계정은 카드로 모여 있고, 스위치 하나로 백그라운드 동기화를 켭니다.',
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
        '<code>.eml</code> 파일, 폴더, <code>.zip</code>, <code>.mbox</code>(Google 테이크아웃, Thunderbird), Outlook <code>.pst</code>, IMAP 메일함, 그리고 AWS·MinIO·R2·B2의 S3 버킷. 중복은 해시로 걸러냅니다.',
      'f.search.t': '한글에 강한 검색',
      'f.search.b':
        '보낸 사람, 제목, 본문, 첨부파일 이름을 한 번에 검색하고 <code>from:</code>, <code>has:attachment</code> 같은 필터도 쓰고 저장해 둘 수 있습니다. 한국어·일본어·중국어가 그대로 검색되고, 초성만 입력해도(<code>ㅎㄱ</code> → 한국) 찾아냅니다.',
      'f.private.t': '내 정보는 내 컴퓨터에만',
      'f.private.b':
        '내 컴퓨터(<code>127.0.0.1</code>)에서 오는 접속만 받습니다. 계정도, 사용 기록 수집도, 클라우드도 없습니다. IMAP 비밀번호는 직접 켜지 않는 한 저장하지 않고, 켜더라도 암호화하며 그 키는 운영체제의 보안 저장소(없으면 내 계정만 읽을 수 있는 파일)에 둡니다.',
      'f.safe.t': 'HTML도 안전하게',
      'f.safe.b': '메일은 격리된 영역에 표시되고 외부 이미지는 기본으로 막혀 있어서, 메일을 열었는지 몰래 확인하는 추적 이미지가 동작하지 않습니다.',
      'f.sync.t': '새 메일도 놓치지 않게',
      'f.sync.b': 'IMAP 프로필에 백그라운드 동기화를 켜 두면, IMAP IDLE을 지원하는 서버에서는 새 메일이 몇 초 안에 들어오고 데스크톱 알림도 받을 수 있습니다. 읽기 전용이라 메일함은 절대 건드리지 않습니다.',
      'f.export.t': '언제든 들고 나갈 수 있게',
      'f.export.b': '라이브러리 전체를 <code>.zip</code> 하나로 내려받거나 S3에 올리고, 검색 결과만 골라 내보낼 수도 있습니다. 자동 백업은 매일 또는 매주 새 메일을 폴더에 복사합니다.',
      'f.attach.t': '첨부파일을 한곳에서',
      'f.attach.b': '모든 메일의 첨부파일이 한 표에. 이름, 종류, 보낸 사람, 날짜로 거르고, 크기순으로 정렬하고, 한글(HWP)·워드·엑셀 파일도 앱 안에서 바로 열어 봅니다.',
      'f.stats.t': '한눈에 보는 통계',
      'f.stats.b': '기간별 메일 수 그래프, 요일·시간대별로 메일이 오는 때, 많이 보낸 사람과 도메인, 용량을 차지하는 것까지 보여 줍니다.',
      'f.tidy.t': '정리하고 인쇄하기',
      'f.tidy.b': '여러 메일을 골라 한꺼번에 읽음·안 읽음으로 바꾸거나 지웁니다. 메일을 인쇄하거나 PDF로 저장하고, 두 번 들어온 메일은 하나로 합칩니다.',
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
