# EnvLock (`el`)

[![CI](https://github.com/itskdhere/envlock/actions/workflows/ci.yml/badge.svg)](https://github.com/itskdhere/envlock/actions/workflows/ci.yml)
&nbsp;
[![Release](https://github.com/itskdhere/envlock/actions/workflows/release.yml/badge.svg)](https://github.com/itskdhere/envlock/actions/workflows/release.yml)
&nbsp;
[![Version](https://img.shields.io/github/v/release/itskdhere/envlock?logo=semver&label=Version)](https://github.com/itskdhere/envlock/releases/latest)

Zero-cloud, tamper-evident asymmetric envelope encryption for environment secrets.  
Keep your `.env` files local and uncommitted while securely checking encrypted snapshots (`env.lock`) directly into Git.

## 🚀 Key Features

- **Zero Cloud & Zero SaaS:** Pure Curve25519 (X25519 ECDH) asymmetric key exchange and AES-256-GCM envelope encryption. No cloud accounts, tokens, or servers.
- **Sub-10ms Latency:** Instantaneous push/pull operations with hardware-accelerated standard library cryptography.
- **Byte-for-Byte Fidelity:** Preserves 100% of formatting, comments (`#`), section dividers, multiline values, and custom quoting.
- **Process-Table Hardened:** Strictly disallows passing private keys as CLI argument strings to prevent leaking secrets to `ps aux` or shell histories.
- **Ergonomic Short Alias (`el`):** Ships with both `envlock` and the short alias `el` out of the box.

## ⚡ Quick Install / Update

### macOS & Linux

```bash
curl -fsSL https://envlock.itskdhere.com | bash
```

### Windows (PowerShell)

```powershell
irm https://envlock.itskdhere.com | iex
```

### Go Developers

```bash
go install github.com/itskdhere/envlock@latest
```

> [!TIP]
> **Prefer direct scripts or want to inspect before running?**
>
> - **macOS / Linux:** [`https://envlock.itskdhere.com/install.sh`](https://envlock.itskdhere.com/install.sh) (alias: `/sh`)
> - **Windows:** [`https://envlock.itskdhere.com/install.ps1`](https://envlock.itskdhere.com/install.ps1) (alias: `/ps1`)

## 🛠️ Quick Start

### 1. Initialize your repository

```bash
envlock init
# or: el init
```

Adds safety rules to `.gitignore` (ignoring `.env`, `*.key`, and `*.pub` while keeping `!env.lock` and `!env.*.lock` tracked) and `.gitattributes` (binary merge protection).

### 2. Generate your identity keypair

```bash
envlock keygen
# or: el keygen
```

Creates `envlock.key` (mode 0600, owner-only) and `envlock.pub`.

> **Tip:** Use `envlock keygen --global` (or `-g`) to store your keypair centrally in your user config directory (`~/.config/envlock/keys/` or `%APPDATA%\envlock\keys\`) across all projects.

### 3. Encrypt local `.env` into `env.lock`

```bash
envlock push
# or: el push
```

Encrypts your plaintext `.env` into a tamper-evident `env.lock` file. You can safely commit `env.lock` to Git.

### 4. Decrypt `env.lock` into local `.env`

```bash
envlock pull
# or: el pull
```

Restores the exact `.env` file byte-for-byte. If local unpushed changes exist, EnvLock warns you before overwriting (`--force` / `-y` to bypass).

### 5. Run commands with secrets injected in memory

```bash
envlock run -- npm start
# or: el run -e prod -- ./deploy.sh
```

Decrypts `env.lock` in memory and runs the command with its variables in the environment. No plaintext `.env` is ever written to disk. Vault variables override host variables unless you pass `--no-override`. Stdin/stdout/stderr, `Ctrl+C`, and the exit code pass straight through.

## ⚙️ CI/CD (Zero-Disk)

Store your private key as a CI secret and expose it as `ENVLOCK_KEY`:

```yaml
- run: envlock run -- npm test
  env:
    ENVLOCK_KEY: ${{ secrets.ENVLOCK_KEY }}
```

If no key can be resolved, EnvLock exits immediately with code 1 instead of waiting for input.

## 📁 Multi-Environment Support

Work seamlessly with multiple environments using the `-e` / `--env` flag:

```bash
# Encrypt .env.staging -> env.staging.lock
el push -e staging

# Decrypt env.staging.lock -> .env.staging
el pull -e staging

# Production
el push -e prod
el pull -e prod
```

## 📖 Command Reference

| Command           | Alias            | Description                                                                               |
| :---------------- | :--------------- | :---------------------------------------------------------------------------------------- |
| `envlock init`    | `el init`        | Prepares project safety defaults (`.gitignore`, `.gitattributes`, sample `.env`)          |
| `envlock keygen`  | `el keygen`      | Generates a Curve25519 X25519 keypair (`--global`, `--out`, `--stdout`, `-y` / `--force`) |
| `envlock push`    | `el push`, `enc` | Encrypts raw `.env` bytes into `env.lock` for all authorized recipients                   |
| `envlock pull`    | `el pull`, `dec` | Decrypts `env.lock` with uncommitted changes warning/prompt (`-y` / `--force`)            |
| `envlock run`     | `el run`         | Runs `-- <command>` with vault variables injected in memory (`--no-override`)             |
| `envlock version` | `el version`     | Prints semantic version, commit hash, and target architecture                             |

## 🔒 Security & Verification

All official release binaries and archives are signed with **cryptographic build provenance** using Sigstore and GitHub Artifact Attestations.

You can verify the authenticity and integrity of any downloaded binary or release asset using the GitHub CLI:

```bash
gh attestation verify <path-to-binary-or-archive> --repo itskdhere/envlock
```

#### Examples:

```bash
# Verify your installed binary (macOS / Linux)
gh attestation verify $(which envlock) --repo itskdhere/envlock

# Verify Windows binary (PowerShell)
gh attestation verify (Get-Command envlock.exe).Source --repo itskdhere/envlock

# Verify a downloaded release archive
gh attestation verify envlock_*_linux_amd64.tar.gz --repo itskdhere/envlock
```
