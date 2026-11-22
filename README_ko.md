# Local Eml

[English](README.md) · [한국어](README_ko.md) · **[문서](https://hwhang0917.github.io/local-eml/docs/ko/)**

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/hwhang0917/local-eml/total)](https://github.com/hwhang0917/local-eml/releases)

Local Eml은 오래된 메일을 내 컴퓨터에 보관해 두고 인터넷 없이 검색하고 읽을 수 있게 해 주는 프로그램입니다. `.eml` 파일이나 폴더, `.zip`, `.mbox`, Outlook `.pst`를 넣을 수 있고, IMAP 계정이나 S3 버킷에서 메일을 가져올 수도 있습니다. 메일은 내 컴퓨터 밖으로 나가지 않습니다.

메일 보관용 도구입니다. 메일함이 꽉 찼을 때 오래된 메일을 `.eml` 파일로 옮겨 두면, 나중에 필요할 때 언제든 찾아볼 수 있습니다.

메일 프로그램은 아닙니다. 메일을 쓰거나 보내거나 답장할 수 없고, 메일함의 내용은 전혀 바꾸지 않습니다. IMAP으로는 읽기만 합니다. 데스크톱 전용이며, 내 컴퓨터에서만 접속할 수 있습니다.

## 다운로드

[최신 릴리스](https://github.com/hwhang0917/local-eml/releases/latest)에서 운영체제에 맞는 파일을 받아 더블클릭하면 됩니다. 전용 창이 열리고, 창을 닫으면 프로그램도 꺼집니다. 따로 설치하거나 가입할 필요가 없습니다. Windows와 macOS에서는 처음 열 때 경고가 뜨는데, 넘어가는 방법은 [설치 안내](https://hwhang0917.github.io/local-eml/docs/ko/install)에 있습니다.

Windows에서는 winget 등록 심사가 끝나면 `winget install hwhang0917.LocalEml`로도 설치할 수 있습니다.

항상 켜 두고 쓰고 싶다면 백그라운드 서비스로 설치하세요. 컴퓨터를 켜면 함께 실행되고, IMAP 계정의 새 메일도 계속 받아 올 수 있습니다.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

설치가 끝나면 <http://localhost:7878>에 접속하세요. 처음에는 세션 토큰을 묻는데, `local-eml token` 명령으로 토큰을 받는 방법이 화면에 나옵니다. 실행 파일을 더블클릭해도 되는데, 이때는 토큰 없이 바로 열립니다. 포트 바꾸기, 내려받은 파일 확인, 업데이트, 삭제 방법은 [설치 안내](https://hwhang0917.github.io/local-eml/docs/ko/install)를 참고하세요.

## 문서

따라 하기:

- [.eml 파일 열기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/open-eml-files)
- [Google 테이크아웃 .mbox 보기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/takeout-mbox)
- [Outlook 없이 .pst 열기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/pst-without-outlook)
- [Gmail 용량 확보하기](https://hwhang0917.github.io/local-eml/docs/ko/how-to/free-up-gmail)

[전체 문서](https://hwhang0917.github.io/local-eml/docs/ko/)에는 [메일 가져오기](https://hwhang0917.github.io/local-eml/docs/ko/import), 초성검색을 포함한 [검색](https://hwhang0917.github.io/local-eml/docs/ko/search), [새 메일 받기](https://hwhang0917.github.io/local-eml/docs/ko/sync), [스팸·피싱 경고](https://hwhang0917.github.io/local-eml/docs/ko/spam-phishing), [데이터 보관 위치](https://hwhang0917.github.io/local-eml/docs/ko/privacy), [설정과 명령어](https://hwhang0917.github.io/local-eml/docs/ko/settings-cli), [문제 해결](https://hwhang0917.github.io/local-eml/docs/ko/troubleshooting)이 정리되어 있습니다.

## 기여하기

소스에서 빌드하고 실행하는 방법은 [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)에 있습니다. 이슈나 풀 리퀘스트를 올리기 전에 [`CONTRIBUTING.md`](CONTRIBUTING.md)를 읽어 주세요. 사용한 오픈소스 라이브러리의 라이선스는 앱의 **설정 → 오픈소스 라이선스**에서 볼 수 있습니다.

## 라이선스

[GNU General Public License v3.0](LICENSE)
