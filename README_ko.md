# Local Eml

[English](README.md) · [한국어](README_ko.md)

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

내 PC에서 `.eml` 파일을 보관하고 읽는 개인용 뷰어입니다. 폴더나 `.zip`을 끌어다 놓아도 되고 S3나 IMAP 메일함에서 가져올 수도 있습니다. 모든 데이터는 내 PC를 벗어나지 않습니다.

## 어떤 도구인가요?

Local Eml은 이메일을 로컬에 보관해 두고 꺼내 보는 **아카이브 도구**입니다. 메일함을 가득 채운 오래된 메일을 `.eml` 파일 그대로 내 PC에 옮겨 두면, 계정 용량은 비우면서도 필요할 때 언제든 검색해서 읽을 수 있습니다.

다만 Thunderbird나 Outlook 같은 이메일 클라이언트는 **아닙니다**. 메일을 쓰거나 보내거나 답장할 수 없고 서버의 메일함을 건드리지도 않습니다 — IMAP 연결도 복사본만 가져오는 읽기 전용입니다. 평소 메일은 쓰던 클라이언트에서 그대로 보고, 오래 남겨 두고 싶은 메일만 Local Eml에 보관하세요.

## 설치

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

설치 스크립트가 최신 릴리스를 내려받아 무결성을 확인한 뒤, 부팅 시 자동으로 시작되도록 백그라운드 서비스로 등록합니다. 설치가 끝나면 브라우저에서 <http://localhost:7878> 을 열어 사용하세요.

### 설치 없이 더블클릭으로 실행하기

터미널이 익숙하지 않다면 [최신 릴리스](https://github.com/hwhang0917/local-eml/releases/latest)에서 내 운영체제에 맞는 파일을 내려받아 더블클릭하세요. Local Eml이 전용 브라우저 창으로 열리고, 창을 닫으면 몇 분 뒤 스스로 종료되어 백그라운드에 아무것도 남지 않습니다.

이 방식에서는 창이 열려 있는 동안에만 새 메일을 받아옵니다. 항상 메일을 받아오게 하려면 위의 설치 스크립트를 사용하세요.

## 주요 기능

- **가져오기**: `.eml` 파일, 폴더, `.zip` 압축 파일, **AWS S3** 버킷, **IMAP** 메일함에서 메일을 가져옵니다. 파일 해시로 중복을 걸러내 같은 메일은 두 번 들어오지 않고 진행 중인 가져오기는 언제든 취소할 수 있습니다.
- **검색**: 보낸 사람, 제목, 본문을 한 번에 검색합니다. 한글을 비롯한 CJK 언어도 잘 검색되고 초성만 입력하는 **초성검색**(예: `ㅎㄱ` → `한국`)도 라이브러리 전체에서 동작합니다.
- **새 메일 자동 수신**: IMAP 프로필별로 켜 두면 마지막 동기화 이후 도착한 메일만 자동으로 받아옵니다 (기본 10분 간격, 가져오기 화면에서 변경 가능).
- **안전한 미리보기**: HTML 메일은 격리된 iframe 안에 표시되고 외부 이미지는 기본적으로 차단됩니다.
- **별표**: 다시 볼 메일에 별표를 붙여 두고 "별표한 메일만 보기"로 골라 볼 수 있습니다.
- **내보내기**: 라이브러리 전체를 `.zip` 하나로 내려받거나 S3 버킷에 올릴 수 있습니다. 이미 올라가 있는 파일은 건너뛰므로 여러 번 실행해도 안전합니다.
- **프로필 저장**: IMAP과 S3 접속 정보에 이름을 붙여 저장해 두면 매번 다시 입력할 필요가 없습니다.
- **한국어 / 영어** 인터페이스와 **절대 / 상대** 날짜 표시를 설정에서 바꿀 수 있습니다.

데이터는 `~/.local-eml/` (Windows에서는 `%USERPROFILE%\.local-eml\`)에 저장됩니다. Local Eml은 `127.0.0.1`, 즉 내 PC에서만 접속할 수 있고 네트워크에는 전혀 노출되지 않습니다. 백그라운드 서비스가 응답하지 않으면 화면 상단에 빨간 안내 바가 나타납니다.

## 성능

메일이 많아져도 괜찮을까요? 중급 노트북(12세대 i5, WSL2)에서 가상 메일 1만 건·10만 건을 만들어 측정한 결과입니다. 목록 한 페이지(50건, 전체 건수 집계 포함)를 불러오는 데 걸린 시간:

| 작업 | 메일 1만 건 | 메일 10만 건 |
|---|---|---|
| 목록 탐색 / 페이지 이동 | 1 ms 미만 | 1 ms 미만 |
| 검색 (일반적인 검색어) | 1 ms 미만 | 약 2 ms |
| 검색 (거의 모든 메일에 들어 있는 단어) | 약 20 ms | 약 200 ms |
| 초성검색 | 약 5 ms | 약 50 ms |
| 대화별로 묶은 목록 | 약 85 ms | 약 0.9초 |
| 대화별 묶기 + 검색 | 약 100 ms | 약 1초 |

목록 탐색과 전문 검색(SQLite FTS5)은 10만 건을 훌쩍 넘어도 사실상 바로 응답합니다. 메일이 많아질수록 느려지는 것은 대화별로 묶은 목록뿐이니, 라이브러리가 아주 커져서 페이지가 굼뜨게 느껴지면 묶기 토글만 끄면 됩니다. 직접 측정해 보려면:

```bash
go test ./internal/store -bench ListEmails -benchtime 5x -run '^$'
LOCAL_EML_BENCH_N=100000 go test ./internal/store -bench ListEmails -benchtime 5x -run '^$'
```

## 비밀번호와 키는 어디에 저장되나요?

- **AWS S3 시크릿 키, 세션 토큰**: 저장하지 않습니다. 가져오기 / 내보내기 때마다 다시 입력합니다.
- **IMAP 비밀번호**: 기본적으로 저장하지 않습니다. 가져오기 때마다 다시 입력하세요.
  - 저장된 IMAP 프로필에서 **"새 메일을 백그라운드에서 받아오기"** 를 켜면, 사용자가 자리에 없어도 Local Eml이 스스로 로그인해야 합니다. 이때 비밀번호는 AES-256-GCM으로 암호화해 데이터베이스에 저장하고 암호화 키는 별도 파일(`~/.local-eml/keys/secret.key`, 권한 `0600`)에 보관합니다. 데이터베이스가 유출되더라도 키 파일이 없으면 비밀번호를 복호화할 수 없습니다. 토글을 끄면 저장된 비밀번호와 동기화 상태도 함께 삭제됩니다.
- **그 외 프로필 정보**(호스트, 버킷, 리전, 사용자명, Access Key ID 등): 비밀이 아니므로 SQLite에 그대로 저장됩니다.

## 제거

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

데이터 폴더까지 함께 지우려면 `--purge` (Windows는 `-Purge`)를 덧붙이세요. 옵션 없이 제거하면 메일 라이브러리는 그대로 남습니다.

## 직접 빌드하기

빌드·실행·기여 방법은 [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) 를 참고하세요. 사용된 오픈소스 라이브러리는 앱 내 **설정 → 오픈소스 라이선스** 에서 확인할 수 있습니다.

## 라이선스

[GNU General Public License v3.0](LICENSE)
