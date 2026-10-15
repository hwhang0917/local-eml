# 설치

Local Eml은 실행 파일 하나입니다. 실행 방법은 세 가지이며, 쓰는 방식에 맞게 고르세요.

| | 더블클릭 | 백그라운드 서비스 | winget |
|---|---|---|---|
| 터미널 필요 | 아니요 | 설치할 때 한 번 | 설치할 때 한 번 |
| PC와 함께 시작 | 아니요 | 예 | 아니요 |
| 새 IMAP 메일을 항상 받아옴 | 창이 열려 있는 동안만 | 예 | 창이 열려 있는 동안만 |

## 내려받아 더블클릭

가장 간단하고 대부분의 사람에게 맞는 방법입니다. [최신 릴리스](https://github.com/hwhang0917/local-eml/releases/latest)에서 내 운영체제에 맞는 파일을 내려받아 여세요. Local Eml이 전용 브라우저 창으로 열립니다. 창을 닫으면 몇 분 뒤 스스로 종료되어 백그라운드에 아무것도 남지 않습니다.

**Windows:** `local-eml-windows-amd64.exe` (ARM 노트북은 `-arm64`). 처음 열 때 **"Windows의 PC 보호"** 창이 뜰 수 있습니다. Microsoft의 평판 서비스에 아직 알려지지 않은 새 프로그램이면 모두 나오는 안내입니다. **추가 정보**를 누른 뒤 **실행**을 선택하세요.

**macOS:** Apple silicon은 `local-eml-darwin-arm64`, Intel은 `local-eml-darwin-amd64`. macOS는 App Store 밖에서 온 공증되지 않은 프로그램을 개발자를 확인할 수 없다는 메시지와 함께 막습니다. **시스템 설정 → 개인정보 보호 및 보안**을 열어 아래쪽의 Local Eml 안내에서 **그래도 열기**를 누른 뒤 파일을 다시 여세요. macOS 14 이하에서는 파일을 오른쪽 클릭해 **열기**를 선택하면 됩니다.

**Linux:** `local-eml-linux-amd64` 또는 `-arm64`. `chmod +x`로 실행 권한을 주고 실행하세요. 기본 브라우저에서 열립니다.

이 방식에서는 창이 열려 있는 동안에만 새 메일을 받아옵니다. 백그라운드 서비스가 이미 실행 중이면 더블클릭은 그 서비스의 창을 열어 줍니다.

더블클릭할 때마다 깨끗하게 시작하고 흔적을 남기지 않으려면 [임시 세션](./privacy#임시-세션)을 보세요.

## 백그라운드 서비스 (Linux, macOS, Windows)

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

설치 스크립트가 최신 릴리스를 내려받아 체크섬을 확인한 뒤, 자동으로 시작되는 백그라운드 서비스로 등록합니다. 그다음 <http://localhost:7878> 을 여세요.

다른 포트를 쓰려면 설치 전에 `LOCAL_EML_PORT`를 지정하세요. 예: `LOCAL_EML_PORT=9000 curl … | sh`, PowerShell에서는 `$env:LOCAL_EML_PORT=9000`. 어느 포트든 내 PC에서만 접속할 수 있습니다.

## winget (Windows)

```powershell
winget install hwhang0917.LocalEml
```

포터블 실행 파일을 설치하고 `local-eml` 명령을 추가합니다. 업데이트는 `winget upgrade`로 받으며, 이렇게 설치한 복사본에서는 앱 내 업데이터가 비켜섭니다.

::: info
winget 등록은 커뮤니티 저장소에서 심사 중입니다. 승인 전까지는 위의 방법을 사용하세요.
:::

## 업데이트

- **서비스 설치**는 **설정 → 정보**에서 업데이트를 확인하고 그 자리에서 설치할 수 있습니다. 서비스가 스스로 재시작합니다.
- **더블클릭** 사용자는 릴리스 페이지에서 새 파일을 내려받습니다.
- **winget** 사용자는 `winget upgrade hwhang0917.LocalEml`을 실행합니다.

## 내려받은 파일 검증하기

모든 릴리스 파일에는 이 저장소의 릴리스 워크플로가 태그된 커밋에서 빌드했음을 증명하는 [빌드 출처 증명(attestation)](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations)이 붙어 있습니다.

```bash
gh attestation verify local-eml-linux-amd64 --repo hwhang0917/local-eml
```

각 릴리스의 `SHA256SUMS` 파일에는 같은 파일들의 체크섬이 들어 있습니다.

## 제거

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

데이터 폴더까지 지우려면 `--purge` (Windows는 `-Purge`)를 덧붙이세요. 옵션 없이 제거하면 메일 라이브러리는 디스크에 그대로 남습니다.
