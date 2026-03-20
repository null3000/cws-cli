# cws — Chrome Web Store CLI

A single-binary CLI for managing Chrome Web Store extensions. Upload, publish, rollout, and more — from your terminal, powered by the latest V2 API.

**[Documentation](https://null3000.github.io/cws-cli/)**

## Install

```bash
brew install null3000/tap/cws
```

Or via script:

```bash
curl -fsSL https://null3000.github.io/cws-cli/install.sh | bash
```

Or from source:

```bash
go install github.com/null3000/cws-cli/cmd/cws@latest
```

## Quick Start

```bash
cws init          # interactive credential setup
cws upload ./dist # zip and upload
cws publish       # publish to the store
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

## Why cws?

| | `cws` | `chrome-webstore-upload-cli` |
|---|---|---|
| **Runtime** | Single binary — no dependencies | Requires Node.js + npm |
| **API version** | Chrome Web Store API **V2** | V1 ([migration requested](https://github.com/fregante/chrome-webstore-upload/issues/114)) |
| **Setup** | Interactive `cws init` wizard | Manual env var configuration |
| **Commands** | upload, publish, status, rollout, cancel | upload, publish |
| **Config** | TOML file + env vars + CLI flags | Env vars only |
| **CI/CD** | Drop in a binary — no `npm install` step | Requires Node.js in your CI image |

## License

MIT
