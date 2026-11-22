# 설치

Local Eml은 실행 파일 하나로 되어 있습니다. 실행하는 방법은 세 가지입니다.

| | 더블클릭 | 백그라운드 서비스 | winget |
|---|---|---|---|
| 터미널 사용 | 필요 없음 | 설치할 때 한 번 | 설치할 때 한 번 |
| 컴퓨터를 켜면 자동 실행 | 아니요 | 예 | 아니요 |
| 새 IMAP 메일 받기 | 창이 열려 있을 때만 | 항상 | 창이 열려 있을 때만 |

잘 모르겠다면 첫 번째 방법을 쓰세요.

## 내려받아 더블클릭하기

[최신 릴리스](https://github.com/hwhang0917/local-eml/releases/latest)에서 운영체제에 맞는 파일을 내려받아 여세요. Local Eml이 전용 브라우저 창으로 열립니다. 창을 닫으면 몇 분 뒤에 프로그램도 꺼지고, 백그라운드에 남는 것은 없습니다.

**Windows:** `local-eml-windows-amd64.exe`를 받으세요. ARM 노트북이라면 `-arm64`입니다. 처음 열 때 **"Windows의 PC 보호"** 창이 뜰 수 있습니다. 아직 많이 쓰이지 않은 새 프로그램이면 어떤 것이든 뜨는 창입니다. **추가 정보**를 누르고 **실행**을 누르세요.

**macOS:** Apple 실리콘 Mac은 `local-eml-darwin-arm64.app.zip`, Intel Mac은 `local-eml-darwin-amd64.app.zip`을 받으세요. 압축을 풀고 **Local Eml**을 더블클릭합니다. App Store에서 받은 앱이 아니고 Apple의 공증도 받지 않았기 때문에, 개발자를 확인할 수 없다는 메시지가 뜹니다. **시스템 설정 → 개인정보 보호 및 보안**을 열고 아래로 내려 Local Eml 관련 메시지에서 **그래도 열기**를 누른 다음, 앱을 다시 여세요. macOS 14 이하에서는 앱을 오른쪽 클릭하고 **열기**를 고르면 됩니다.

**Linux:** `local-eml-linux-amd64` 또는 `-arm64`를 받아 `chmod +x`로 실행 권한을 준 뒤 실행하세요. 기본 브라우저에서 열립니다.

이 방법으로 쓰면 새 메일은 창이 열려 있을 때만 받아 옵니다. 백그라운드 서비스가 이미 실행 중이라면, 더블클릭했을 때 프로그램을 하나 더 띄우지 않고 서비스 화면을 엽니다.

열 때마다 빈 상태로 시작하고 아무것도 남기지 않으려면 [임시 세션](./privacy#임시-세션)을 참고하세요.

## 백그라운드 서비스 (Linux, macOS, Windows)

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

설치 스크립트가 최신 릴리스를 내려받아 파일이 손상되거나 바뀌지 않았는지 확인하고, 컴퓨터를 켤 때 자동으로 실행되도록 등록합니다. 설치가 끝나면 <http://localhost:7878>에 접속하세요. 처음에는 세션 토큰을 입력하라고 나옵니다. `local-eml token`을 실행하면 토큰이 나오니 그대로 붙여 넣으면 됩니다. 실행 파일을 더블클릭해도 되는데, 이때는 자동으로 로그인됩니다. 토큰이 왜 필요한지는 [세션 토큰](./settings-cli#세션-토큰)에 설명해 두었습니다.

7878이 아닌 다른 포트를 쓰려면 스크립트를 실행하기 전에 `LOCAL_EML_PORT`를 지정하세요. 예를 들어 `LOCAL_EML_PORT=9000 curl … | sh`, PowerShell에서는 `$env:LOCAL_EML_PORT=9000`입니다. 어떤 포트를 쓰든 내 컴퓨터에서만 접속할 수 있습니다.

## winget (Windows)

```powershell
winget install hwhang0917.LocalEml
```

프로그램을 설치하고 `local-eml` 명령을 쓸 수 있게 해 줍니다. 업데이트는 `winget upgrade`로 하기 때문에, 이렇게 설치하면 앱 안의 업데이트 기능은 꺼집니다.

::: info
winget 등록은 아직 심사 중입니다. 승인되기 전까지는 위의 방법을 쓰세요.
:::

## 업데이트

- 백그라운드 서비스: **설정 → 정보**에서 업데이트를 확인하고 설치하면 됩니다. 서비스는 알아서 다시 시작됩니다.
- 더블클릭: 릴리스 페이지에서 새 파일을 받아 기존 파일 대신 쓰세요.
- winget: `winget upgrade hwhang0917.LocalEml`을 실행하세요.

## 내려받은 파일 확인하기

내려받은 파일이 정말 이 프로젝트의 릴리스 과정에서 만들어졌는지 확인할 수 있습니다. 릴리스 파일마다 [빌드 출처 증명(attestation)](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations)이 붙어 있어서, GitHub CLI로 검사하면 됩니다.

```bash
gh attestation verify local-eml-linux-amd64 --repo hwhang0917/local-eml
```

릴리스마다 모든 파일의 체크섬을 적은 `SHA256SUMS` 파일도 들어 있습니다.

## 삭제

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

프로그램과 서비스만 지우고 메일은 그대로 남겨 둡니다. 메일과 설정까지 모두 지우려면 아래 명령을 쓰세요.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh -s -- --purge
```

```powershell
# Windows
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1))) -Purge
```
