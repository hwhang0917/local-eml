# 문제 해결

## 서버가 응답하지 않는다는 빨간 배너가 보여요

페이지는 열려 있지만 백그라운드 프로세스가 멈춘 상태입니다. 서비스 설치라면 재시작하세요.

```bash
# Linux
systemctl --user restart local-eml
# macOS
launchctl kickstart -k gui/$(id -u)/local-eml
```

Windows에서는 서비스 관리자에서 **Local Eml** 서비스를 재시작하세요. 더블클릭 모드라면 실행 파일을 다시 열면 됩니다.

## "창을 더 넓혀 주세요"라고 나와요

창이 인터페이스에 필요한 폭보다 좁습니다. 창을 넓히거나 최대화하세요. 인쇄할 때 이 화면이 나오면 v0.15.1 이상으로 업데이트하세요. 그 버전부터 인쇄 시에는 나오지 않습니다.

## 7878 포트를 이미 다른 프로그램이 쓰고 있어요

다른 포트로 설치하세요. 설치 전에 `LOCAL_EML_PORT`를 지정합니다. 직접 실행할 때는 `local-eml serve --port 9000`입니다.

## 더블클릭하면 별도 창이 아니라 서비스가 열려요

의도된 동작입니다. 서비스가 실행 중이면 더블클릭은 두 번째 복사본을 띄우지 않고 서비스의 창을 엽니다. 서비스의 라이브러리를 건드리지 않는 독립 세션이 필요하면 [임시 세션](./privacy#임시-세션)을 사용하세요.

## 업데이터가 winget을 쓰라고 해요

지금 실행 중인 복사본은 winget으로 설치된 것이며, winget이 버전을 관리합니다. 다음으로 업데이트하세요.

```powershell
winget upgrade hwhang0917.LocalEml
```

## 정상 메일인데 피싱 경고가 붙어요

경고는 휴리스틱입니다. 배너에서 **스팸 아님**을 선택하면 그 메일의 경고만 사라지고, **설정 → 스팸·피싱**에서 경고를 모두 끌 수 있습니다. [스팸과 피싱](./spam-phishing)을 보세요.

## 로그는 어디에 있나요

```bash
tail -f ~/.local-eml/logs/local-eml.log
```

Windows에서는 `%USERPROFILE%\.local-eml\logs\local-eml.log`입니다. 더 자세한 로그가 필요하면 서비스 환경에 `LOCAL_EML_LOG_LEVEL=debug`를 지정하세요. 로그에는 제목과 주소가 들어 있으니 공유하기 전에 가려 주세요.

## 버그 신고

[github.com/hwhang0917/local-eml/issues](https://github.com/hwhang0917/local-eml/issues)에 버전(**설정 → 정보**), 운영체제, 실행 방식, 관련 로그 줄을 적어 이슈를 열어 주세요.
