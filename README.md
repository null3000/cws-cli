# cws — Chrome Web Store CLI

A command-line tool for managing Chrome Web Store extensions using the [V2 API](https://developer.chrome.com/docs/webstore/api/reference/rest). Supports uploading, publishing, staged rollouts, and status checks. Designed for automation in CI/CD pipelines.

## Installation

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/null3000/cws-cli/releases).

```bash
# macOS (Apple Silicon)
curl -LO https://github.com/null3000/cws-cli/releases/latest/download/cws_darwin_arm64.tar.gz
tar xzf cws_darwin_arm64.tar.gz
sudo mv cws /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/null3000/cws-cli/releases/latest/download/cws_darwin_amd64.tar.gz
tar xzf cws_darwin_amd64.tar.gz
sudo mv cws /usr/local/bin/

# Linux (amd64)
curl -LO https://github.com/null3000/cws-cli/releases/latest/download/cws_linux_amd64.tar.gz
tar xzf cws_linux_amd64.tar.gz
sudo mv cws /usr/local/bin/

# Linux (arm64)
curl -LO https://github.com/null3000/cws-cli/releases/latest/download/cws_linux_arm64.tar.gz
tar xzf cws_linux_arm64.tar.gz
sudo mv cws /usr/local/bin/

# Windows (amd64) — PowerShell
Invoke-WebRequest -Uri https://github.com/null3000/cws-cli/releases/latest/download/cws_windows_amd64.zip -OutFile cws.zip
Expand-Archive cws.zip -DestinationPath .
Move-Item cws.exe C:\Windows\System32\
```

### From Source

```bash
go install github.com/null3000/cws-cli/cmd/cws@latest
```

## Prerequisites

Before using this CLI, you need to set up API access in the Google Cloud Console. Follow the official guide:

**[Chrome Web Store API — Using the API](https://developer.chrome.com/docs/webstore/using-api)**

Key steps:
1. Create a project in the [Google Cloud Console](https://console.cloud.google.com/)
2. Enable the **Chrome Web Store API**
3. Create an **OAuth consent screen** — set User Type to **Internal** if available (no verification needed). If your Google account is not part of a Google Workspace organization, select **External** instead — you will need to complete Google's verification process.
4. Create **OAuth2 credentials** (Desktop app type)
5. Generate a **refresh token** using the [OAuth Playground](https://developers.google.com/oauthplayground) with scope: `https://www.googleapis.com/auth/chromewebstore`

## Quick Start

Run all `cws` commands from your extension's project directory. This is where `cws init` creates your `cws.toml` config file and where `cws upload` looks for your built extension files.

```bash
cd my-chrome-extension/
```

### 1. Set up credentials

```bash
cws init
```

This interactive wizard will guide you through configuring your OAuth2 credentials, refresh token, and Publisher ID. It writes a `cws.toml` file in the current directory.

### 2. Upload your extension

```bash
cws upload ./dist
```

This zips the directory (excluding `.git/`, `node_modules/`, etc.) and uploads it.

### 3. Publish

```bash
cws publish
```

To see all available commands and flags, run:

```bash
cws help
```

## Commands

| Command | Description |
|---------|-------------|
| `cws init` | Interactive credential setup wizard |
| `cws upload [source]` | Zip and upload a package |
| `cws publish` | Publish the latest uploaded version |
| `cws status` | Check extension status |
| `cws rollout <percentage>` | Set deploy percentage (10k+ users required) |
| `cws cancel` | Cancel a pending submission |
| `cws version` | Print CLI version |
| `cws help` | Show help for any command |

### `cws upload`

```bash
# Upload a directory (zips automatically)
cws upload ./dist

# Upload a pre-built zip
cws upload extension.zip

# Upload and publish in one step
cws upload ./dist --publish

# Specify extension ID
cws upload ./dist -e abcdefghijklmnopabcdefghijklmnop

# Don't wait for processing
cws upload ./dist --wait=false
```

### `cws publish`

```bash
# Publish immediately after review
cws publish

# Stage for review without auto-publishing
cws publish --staged
```

### `cws status`

```bash
# Human-readable output
cws status

# Raw JSON (for scripting)
cws status --json
```

### `cws rollout`

```bash
# Set to 50% rollout
cws rollout 50

# Full rollout
cws rollout 100
```

## Configuration

### Config file (`cws.toml`)

By default, `cws init` writes all credentials to `./cws.toml` in the current directory. **This file contains secrets — do not commit it.** Add it to `.gitignore`.

```toml
publisher_id = "abc1234567890"

[auth]
client_id = "xxxxxxxxxxxx.apps.googleusercontent.com"
client_secret = "GOCSPX-xxxxxxxxxxxx"
refresh_token = "1//xxxxxxxxxxxx"

[extensions.default]
id = "abcdefghijklmnopabcdefghijklmnop"
source = "./dist"
```

Alternatively, use `cws init --global` to write credentials to `~/.config/cws/cws.toml`, keeping secrets out of your project directory. You can then create a minimal per-project `cws.toml` (safe to commit) with just extension settings:

```toml
[extensions.default]
id = "abcdefghijklmnopabcdefghijklmnop"
source = "./dist"
```

### Environment variables

All config values can be set via environment variables:

| Variable | Description |
|----------|-------------|
| `CWS_CLIENT_ID` | OAuth2 Client ID |
| `CWS_CLIENT_SECRET` | OAuth2 Client Secret |
| `CWS_REFRESH_TOKEN` | OAuth2 Refresh Token |
| `CWS_PUBLISHER_ID` | Publisher ID |
| `CWS_EXTENSION_ID` | Default Extension ID |

### Priority order

1. CLI flags (`--extension-id`, etc.)
2. Environment variables (`CWS_*`)
3. Local `cws.toml` (current directory)
4. Global `~/.config/cws/cws.toml`

## CI/CD

### GitHub Actions

```yaml
- name: Upload and publish extension
  env:
    CWS_CLIENT_ID: ${{ secrets.CWS_CLIENT_ID }}
    CWS_CLIENT_SECRET: ${{ secrets.CWS_CLIENT_SECRET }}
    CWS_REFRESH_TOKEN: ${{ secrets.CWS_REFRESH_TOKEN }}
    CWS_PUBLISHER_ID: ${{ secrets.CWS_PUBLISHER_ID }}
  run: |
    cws upload ./dist -e ${{ vars.EXTENSION_ID }} --publish
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (API error, invalid config, etc.) |
| `2` | Timeout waiting for upload processing |

## Why cws?

The go-to tool for Chrome Web Store publishing has been [`chrome-webstore-upload-cli`](https://github.com/fregante/chrome-webstore-upload-cli). It works, but there's room for improvement.

| | `cws` | `chrome-webstore-upload-cli` |
|---|---|---|
| **Runtime** | Single binary — no dependencies | Requires Node.js + npm |
| **API version** | Chrome Web Store API **V2** | V1 ([migration requested](https://github.com/fregante/chrome-webstore-upload/issues/114)) |
| **Setup** | Interactive `cws init` wizard | Manual env var configuration |
| **Commands** | upload, publish, status, rollout, cancel | upload, publish |
| **Config** | TOML file + env vars + CLI flags | Env vars only |
| **CI/CD** | Drop in a binary — no `npm install` step | Requires Node.js in your CI image |

**In short:** `cws` is a single binary with zero dependencies, uses the latest API, and gives you more control over the publishing lifecycle — status checks, staged rollouts, and submission cancellation — without needing Node.js in your environment.

## License

MIT
