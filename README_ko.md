# Local Eml

[English](README.md) · [한국어](README_ko.md)

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)

`.eml` 파일을 위한 개인용 로컬 뷰어입니다. 폴더나 `.zip`을 끌어다 놓거나, S3 / IMAP에서 가져와 검색하고 읽으세요. 모든 데이터는 PC에만 남습니다.

## 설치

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

설치 스크립트는 최신 릴리스를 내려받아 무결성을 확인한 뒤, Local Eml을 백그라운드 서비스로 등록해 자동 실행되도록 합니다. 브라우저에서 <http://localhost:7878> 을 열어 사용하세요.

## 주요 기능

- **가져오기**: `.eml` 파일, 폴더, `.zip` 압축 파일, **AWS S3** 버킷, **IMAP** 메일함에서 메일을 가져옵니다. 파일 해시 기반으로 중복은 자동으로 건너뜁니다.
- **검색**: 보낸 사람·제목·본문을 가로질러 검색합니다. 한국어와 같은 CJK 언어도 자연스럽게 검색됩니다.
- **안전한 미리보기**: HTML 메일은 격리된 iframe 안에서 렌더링되고, 외부 이미지는 기본적으로 차단합니다.
- **별표**: 다시 볼 메일을 별표로 표시해 두세요.
- **내보내기**: 라이브러리 전체를 `.zip` 한 개로 다운로드하거나, S3 버킷으로 다시 업로드할 수 있습니다.
- **프로필 저장**: IMAP과 S3 접속 정보를 이름으로 저장해 두면 매번 다시 입력하지 않아도 됩니다 (비밀번호와 시크릿 키는 저장되지 않습니다).
- **한국어 / 영어** 인터페이스를 설정에서 전환할 수 있습니다.

데이터는 `~/.local-eml/` (Windows에서는 `%USERPROFILE%\.local-eml\`)에 저장됩니다. Local Eml은 `127.0.0.1` 루프백에만 응답하며, 네트워크에 노출되지 않습니다.

## 제거

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

데이터 폴더까지 함께 지우려면 `--purge` (Windows는 `-Purge`)를 덧붙이세요. 옵션 없이 제거하면 메일 라이브러리는 그대로 보존됩니다.

## 직접 빌드하기

빌드·실행·기여 방법은 [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) 를 참고하세요. 사용된 오픈소스 라이브러리는 앱 내 **설정 → 오픈소스 라이선스** 에서 확인할 수 있습니다.
