# Local Eml

[English](README.md) · [한국어](README_ko.md) · **[문서](https://hwhang0917.github.io/local-eml/docs/ko/)**

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/hwhang0917/local-eml/total)](https://github.com/hwhang0917/local-eml/releases)

내 PC에서 `.eml` 파일을 보관하고 읽는 개인용 뷰어입니다. 폴더, `.zip`, `.mbox`, `.pst`를 끌어다 놓아도 되고 S3나 IMAP 메일함에서 가져올 수도 있습니다. 모든 데이터는 내 PC를 벗어나지 않습니다.

Local Eml은 이메일을 로컬에 보관해 두고 꺼내 보는 **아카이브 도구**입니다. 메일함을 가득 채운 오래된 메일을 `.eml` 파일 그대로 내 PC에 옮겨 두고, 필요할 때 언제든 검색해서 읽을 수 있습니다.

이메일 클라이언트는 **아닙니다**. 메일을 쓰거나 보내거나 답장할 수 없고 서버의 메일함을 건드리지도 않습니다 — IMAP 연결은 읽기 전용입니다. PC 전용이며 `127.0.0.1`에서만 접속할 수 있습니다.

## 다운로드

[최신 릴리스](https://github.com/hwhang0917/local-eml/releases/latest)에서 내 운영체제에 맞는 파일을 내려받아 더블클릭하세요. Local Eml이 전용 창으로 열리고, 창을 닫으면 종료됩니다. 설치 프로그램도 계정도 없습니다. Windows와 macOS에서 처음 열 때 나오는 안내는 [설치 가이드](https://hwhang0917.github.io/local-eml/docs/ko/install)에 있습니다.

**winget:** `winget install hwhang0917.LocalEml` (등록 심사 중).

**항상 켜 두고 싶다면** 한 줄로 부팅 시 자동 시작되고 새 IMAP 메일을 계속 받아오는 백그라운드 서비스로 설치할 수 있습니다.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

그다음 <http://localhost:7878> 을 여세요. 포트 변경, 내려받은 파일 검증, 제거, 업데이트는 [설치 가이드](https://hwhang0917.github.io/local-eml/docs/ko/install)에 있습니다.

## 더 알아보기

따라하기: [.eml 파일 열기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/open-eml-files) · [Google 테이크아웃 .mbox 보기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/takeout-mbox) · [Outlook 없이 .pst 열기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/pst-without-outlook) · [Gmail 용량 비우기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/free-up-gmail)

나머지는 모두 **[문서](https://hwhang0917.github.io/local-eml/docs/ko/)**에 있습니다.

- [메일 가져오기](https://hwhang0917.github.io/local-eml/docs/ko/import): 파일, 폴더, 압축 파일, `.mbox`, `.pst`, S3, IMAP
- [검색](https://hwhang0917.github.io/local-eml/docs/ko/search): 초성검색, 필터, 성능
- [새 메일 받기](https://hwhang0917.github.io/local-eml/docs/ko/sync): IMAP 백그라운드 동기화
- [스팸·피싱](https://hwhang0917.github.io/local-eml/docs/ko/spam-phishing): 경고와 표시
- [개인정보와 데이터](https://hwhang0917.github.io/local-eml/docs/ko/privacy): 데이터 위치, 자격 증명, 임시 세션, 모든 데이터 삭제
- [설정과 CLI](https://hwhang0917.github.io/local-eml/docs/ko/settings-cli), [문제 해결](https://hwhang0917.github.io/local-eml/docs/ko/troubleshooting)

## 기여하기

빌드·실행 방법은 [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) 를, 이슈나 풀 리퀘스트를 올리기 전에는 [`CONTRIBUTING.md`](CONTRIBUTING.md) 를 참고하세요. 사용된 오픈소스 라이브러리는 앱 내 **설정 → 오픈소스 라이선스** 에서 확인할 수 있습니다.

## 라이선스

[GNU General Public License v3.0](LICENSE)
