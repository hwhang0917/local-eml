# インストール

Local Emlは1つの実行ファイルだけでできています。実行する方法は3つあります。

| | ダブルクリック | バックグラウンドサービス | winget |
|---|---|---|---|
| ターミナルの使用 | 不要 | インストール時に1回 | インストール時に1回 |
| コンピューターの起動時に自動実行 | いいえ | はい | いいえ |
| 新着IMAPメールの受信 | ウィンドウを開いている間 | 常時 | ウィンドウを開いている間 |

迷ったら、最初の方法を使ってください。

## ダウンロードしてダブルクリック

[最新リリース](https://github.com/hwhang0917/local-eml/releases/latest)からお使いのシステム用のファイルをダウンロードして開きます。Local Emlが専用のブラウザウィンドウで開きます。ウィンドウを閉じると数分後にプログラムも終了し、裏で動き続けるものは何も残りません。

**Windows：** `local-eml-windows-amd64.exe`をダウンロードします（ARMのノートPCでは`-arm64`）。初めて開くとき、「**Windows によって PC が保護されました**」と表示されることがあります。これはまだ利用者の少ない新しいプログラムすべてに表示されるものです。**詳細情報**をクリックし、**実行**をクリックしてください。

**macOS：** Appleシリコン搭載Macでは`local-eml-darwin-arm64.app.zip`、Intel搭載Macでは`local-eml-darwin-amd64.app.zip`をダウンロードします。展開して**Local Eml**をダブルクリックします。App Storeから入手したアプリではなく、Appleの公証も受けていないため、開発元を確認できないというメッセージが表示されます。**システム設定 → プライバシーとセキュリティ**を開き、下にスクロールしてLocal Emlについてのメッセージを探し、**このまま開く**をクリックしてから、もう一度アプリを開いてください。macOS 14以前では代わりにアプリを右クリックして**開く**を選びます。

**Linux：** `local-eml-linux-amd64`または`-arm64`をダウンロードし、`chmod +x`で実行権限を付けて実行します。既定のブラウザで開きます。

この方法で実行した場合、新着メールが取得されるのはウィンドウを開いている間だけです。バックグラウンドサービスがすでに動いている場合、ダブルクリックしても2つ目のプログラムは起動せず、サービスの画面を表示するウィンドウが開きます。

ダブルクリックするたびに空の状態で始め、何も残さないようにしたい場合は、[一時セッション](./privacy#temporary-sessions)を参照してください。

## バックグラウンドサービス（Linux、macOS、Windows） {#background-service}

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows（PowerShell）**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

スクリプトは最新リリースをダウンロードし、ファイルが破損や改ざんを受けていないことを確認したうえで、Local Emlが自動的に起動するように設定します。完了したら<http://localhost:7878>を開いてください。初回はセッショントークンの入力を求められます。取得の手順は画面に表示されます。コマンド プロンプト（Windows）またはターミナル（macOS）を開き、**コピー**ボタンでコピーしたコマンドを貼り付けてEnterキーを押し、表示されたトークンを画面に貼り付けます。実行ファイルをダブルクリックして開くこともでき、その場合は自動的にサインインされます。トークンが必要な理由は[セッショントークン](./settings-cli#session-token)で説明しています。

7878以外のポートを使うには、スクリプトを実行する前に`LOCAL_EML_PORT`を設定します。たとえば`LOCAL_EML_PORT=9000 curl … | sh`、PowerShellでは`$env:LOCAL_EML_PORT=9000`です。どのポートを選んでも、接続できるのは自分のコンピューターだけです。

## winget（Windows）

```powershell
winget install hwhang0917.LocalEml
```

プログラムをインストールし、`local-eml`コマンドを使えるようにします。アップデートは`winget upgrade`で行うため、この方法でインストールした場合、アプリ内のアップデート機能はオフになります。

::: info
wingetへの登録はまだ審査待ちです。承認されるまでは上のいずれかの方法を使ってください。
:::

## アップデート

- バックグラウンドサービス：**設定 → このアプリについて**でアップデートを確認し、インストールします。サービスは自動的に再起動します。
- ダブルクリック：リリースページから新しいファイルをダウンロードし、古いファイルの代わりに使います。
- winget：`winget upgrade hwhang0917.LocalEml`を実行します。

## ダウンロードしたファイルの確認

ダウンロードしたファイルが本当にこのプロジェクトのリリース工程で作られたものかどうかを確認できます。各リリースファイルには[ビルドの来歴証明（attestation）](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations)が付いており、GitHub CLIで検証できます。

```bash
gh attestation verify local-eml-linux-amd64 --repo hwhang0917/local-eml
```

各リリースにはすべてのファイルのチェックサムを記載した`SHA256SUMS`ファイルも含まれています。

## アンインストール

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

プログラムとサービスを削除しますが、メールは残ります。メールと設定も一緒に削除するには、代わりに次のコマンドを使います。

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh -s -- --purge
```

```powershell
# Windows
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1))) -Purge
```
