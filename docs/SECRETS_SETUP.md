# Prompt-Pulse Secrets Setup Guide

**Last Updated**: 2026-02-05
**System**: yoga (Rocky Linux 10)

---

## Overview

Prompt-pulse requires API keys and credentials for rich data ingestion across three categories:
1. **Claude AI** - Usage tracking (OAuth + API keys)
2. **Cloud Billing** - Spend monitoring (Civo, DigitalOcean, AWS, DreamHost)
3. **Infrastructure** - Mesh and cluster monitoring (Tailscale, Kubernetes)

All secrets are managed via **sops-nix** with age encryption. Secret files are decrypted at runtime and exposed via `*_FILE` environment variables.

---

## Current Status on yoga

### ✅ Configured Secrets (via sops-nix)

All secrets are encrypted in `/home/jsullivan2/git/crush-dots/nix/secrets/common.yaml` and decrypted to `~/.config/sops-nix/secrets/` at runtime by home-manager.

| Secret | Path | Status |
|--------|------|--------|
| **API Keys** | | |
| Anthropic API | `api/anthropic` | ✅ Configured |
| Perplexity API | `api/perplexity` | ✅ Configured |
| GitHub Token | `api/github_token` | ✅ Configured |
| GitLab Token | `api/gitlab_token` | ✅ Configured |
| Brave Search | `api/brave` | ✅ Configured |
| Shodan API | `api/shodan` | ✅ Configured |
| **Infrastructure** | | |
| Civo API Key | `infrastructure/civo_api_key` | ✅ Configured |
| DigitalOcean Token | `infrastructure/digitalocean_token` | ✅ Configured |
| DreamHost API Key | `infrastructure/dreamhost_api_key` | ✅ Configured |
| Tailscale API Key | `infrastructure/tailscale_auth_key` | ✅ Configured (but unused - see below) |
| **Database** | | |
| Neon PostgreSQL | `database/neon/connection_string` | ✅ Configured |
| Neon Password | `database/neon/password` | ✅ Configured |
| **Streaming** | | |
| Sunshine API Key | `streaming/sunshine/api_key` | ✅ Configured |

### ✅ OAuth Credentials

| Provider | Path | Status |
|----------|------|--------|
| Claude OAuth | `~/.claude/.credentials.json` | ✅ Configured |

### ✅ Kubernetes Contexts

| Context | Status | Notes |
|---------|--------|-------|
| `do-nyc2-k8s-1-33-1-do-5-nyc2-1760828908532` | ✅ Active | DigitalOcean Kubernetes (NYC2) |
| `tinyland-civo-dev` | ✅ Configured | Civo Kubernetes (fuzzy-dev namespace) |

### ✅ AWS CLI

| Profile | Status | Notes |
|---------|--------|-------|
| `default` | ❌ Not configured | AWS CLI not installed |

### ✅ Tailscale

| Property | Value | Status |
|----------|-------|--------|
| Mesh Status | Online | ✅ Running |
| Tailnet | `taila4c78d.ts.net` | ✅ Detected |
| API Key | Via sops-nix | ⚠️ Not needed (CLI fallback works) |

---

## Secrets Inventory by Collector

### 1. Claude Collector

**Purpose**: Track API usage and rate limits across multiple Claude accounts (subscription + API key types).

#### Required Secrets

| Secret | Type | How to Obtain | Priority |
|--------|------|---------------|----------|
| `~/.claude/.credentials.json` | OAuth credentials | Login via Claude Code CLI: `claude login` | **REQUIRED** |
| `ANTHROPIC_API_KEY` | API key | https://console.anthropic.com/settings/keys | Optional |

#### Environment Variables

```bash
# OAuth Credentials (subscription accounts)
# Path configured in: nix/home-manager/prompt-pulse.nix -> accounts.claude[].credentialsPath
# Default: ~/.claude/.credentials.json

# API Key (for API accounts)
# Variable configured in: config.yaml -> accounts.claude[].api_key_env
# Example:
export ANTHROPIC_API_KEY="sk-ant-api03-..."
```

#### Configuration

```yaml
# ~/.config/prompt-pulse/config.yaml
accounts:
  claude:
    - name: "primary"
      type: "subscription"
      credentials_path: "/home/jsullivan2/.claude/.credentials.json"
      enabled: true
    - name: "work-api"
      type: "api"
      api_key_env: "ANTHROPIC_API_KEY"
      enabled: false  # Enable when API key is added
```

#### Current Status

- ✅ OAuth credentials configured
- ❌ No API key accounts configured
- ✅ Collector functional for subscription account

---

### 2. Billing Collector

**Purpose**: Monitor cloud provider spending with forecasting and budget alerts.

#### Required Secrets

| Provider | Env Var | How to Obtain | Priority |
|----------|---------|---------------|----------|
| **Civo** | `CIVO_API_KEY` or `CIVO_API_KEY_FILE` | https://dashboard.civo.com/security | **REQUIRED** |
| **DigitalOcean** | `DIGITALOCEAN_TOKEN` or `DIGITALOCEAN_TOKEN_FILE` | https://cloud.digitalocean.com/account/api/tokens | **REQUIRED** |
| **AWS** | AWS CLI profile | `aws configure` | Optional |
| **DreamHost** | `DREAMHOST_API_KEY` or `DREAMHOST_API_KEY_FILE` | https://panel.dreamhost.com/?tree=home.api | Optional |

#### Environment Variables

All billing secrets are managed via sops-nix and exposed as `*_FILE` environment variables:

```bash
# Injected by sops-nix (nix/secrets/default.nix)
export CIVO_API_KEY_FILE="$HOME/.config/sops-nix/secrets/infrastructure/civo_api_key"
export DIGITALOCEAN_TOKEN_FILE="$HOME/.config/sops-nix/secrets/infrastructure/digitalocean_token"
export DREAMHOST_API_KEY_FILE="$HOME/.config/sops-nix/secrets/infrastructure/dreamhost_api_key"

# Also available as direct env vars (read from *_FILE by prompt-pulse)
export CIVO_API_KEY="$(cat $CIVO_API_KEY_FILE)"
export DIGITALOCEAN_TOKEN="$(cat $DIGITALOCEAN_TOKEN_FILE)"
export DREAMHOST_API_KEY="$(cat $DREAMHOST_API_KEY_FILE)"
```

#### AWS Configuration

AWS uses AWS CLI profiles instead of environment variables:

```bash
# Install AWS CLI
sudo dnf install awscli2

# Configure default profile
aws configure
# Enter:
#   AWS Access Key ID: (from AWS IAM)
#   AWS Secret Access Key: (from AWS IAM)
#   Default region: us-east-1
#   Default output format: json

# Verify configuration
aws sts get-caller-identity
```

**IAM Permissions Required**:
- `ce:GetCostAndUsage` - Current month spend
- `ce:GetCostForecast` - Forecast remaining month

**Cost Warning**: AWS Cost Explorer charges **$0.01 per API call**. At 15-minute poll intervals, this costs ~$0.72/day. Consider increasing poll interval to 1 hour or disabling AWS billing.

#### Configuration

```yaml
# ~/.config/prompt-pulse/config.yaml
accounts:
  civo:
    api_key_env: "CIVO_API_KEY"
    region: "NYC1"
  digitalocean:
    api_key_env: "DIGITALOCEAN_TOKEN"
  aws:
    profile: "default"
    regions: ["us-east-1"]
  dreamhost:
    api_key_env: "DREAMHOST_API_KEY"
```

#### Current Status

- ✅ Civo API key configured (via sops-nix)
- ✅ DigitalOcean token configured (via sops-nix)
- ✅ DreamHost API key configured (via sops-nix)
- ❌ AWS CLI not installed
- ✅ Billing collector functional for Civo/DO/DreamHost

---

### 3. Tailscale Collector

**Purpose**: Monitor Tailscale mesh network status (online nodes, IPs, last seen).

#### Required Secrets

| Secret | Type | How to Obtain | Priority |
|--------|------|---------------|----------|
| Tailscale API Key | API key | https://login.tailscale.com/admin/settings/keys | Optional (CLI fallback works) |

#### Environment Variables

```bash
# API Key (optional - CLI fallback works)
export TAILSCALE_API_KEY_FILE="$HOME/.config/sops-nix/secrets/infrastructure/tailscale_auth_key"
export TAILSCALE_API_KEY="$(cat $TAILSCALE_API_KEY_FILE)"
```

#### CLI Fallback

Prompt-pulse can use `tailscale status --json` as a fallback when the API key is not available. This works on yoga because Tailscale is running locally.

```bash
# Verify CLI fallback works
tailscale status --json | jq -r '.MagicDNSSuffix'
# Output: taila4c78d.ts.net
```

#### Configuration

```yaml
# ~/.config/prompt-pulse/config.yaml
tailscale:
  tailnet: "tinyland.ts.net"  # Optional - auto-detected from CLI
  api_key_env: "TAILSCALE_API_KEY"
  use_cli_fallback: true  # Enable CLI fallback (default)
  collect_node_metrics: false  # SSH-based metrics (disabled)
```

#### Current Status

- ✅ Tailscale API key configured (via sops-nix, but unused)
- ✅ CLI fallback functional
- ✅ Collector functional via CLI fallback

---

### 4. Kubernetes Collector

**Purpose**: Monitor Kubernetes cluster health (nodes, pods, resource usage).

#### Required Secrets

| Secret | Type | How to Obtain | Priority |
|--------|------|---------------|----------|
| Kubeconfig | File | `kubectl config view --raw > ~/.kube/config` | **REQUIRED** |

#### Environment Variables

```bash
# Kubeconfig path (default)
export KUBECONFIG="$HOME/.kube/config"
```

#### CLI Requirements

Prompt-pulse shells out to `kubectl` for cluster monitoring:

```bash
# Verify kubectl is available
which kubectl
# Output: /nix/store/.../bin/kubectl

# Verify contexts are configured
kubectl config get-contexts
# Output:
# CURRENT   NAME                                         CLUSTER
# *         do-nyc2-k8s-1-33-1-do-5-nyc2-1760828908532   do-nyc2-...
#           tinyland-civo-dev                            tinyland-civo-dev
```

#### Configuration

```yaml
# ~/.config/prompt-pulse/config.yaml
kubernetes:
  contexts:
    - name: "tinyland-civo-dev"
      kubeconfig: ""  # Empty = use KUBECONFIG env var
      namespace: "fuzzy-dev"
      dashboard_url: "https://dashboard.civo.com"
```

#### Current Status

- ✅ kubectl installed (via Nix)
- ✅ Kubeconfig configured
- ✅ 2 contexts available (DO + Civo)
- ✅ Collector functional

---

## How sops-nix Works

### Architecture

```
crush-dots/nix/secrets/common.yaml (encrypted)
          |
          v (decrypted by home-manager at activation)
          |
~/.config/sops-nix/secrets/
          ├── api/
          │   ├── anthropic           (mode 0400, owned by jsullivan2)
          │   ├── perplexity
          │   └── github_token
          ├── infrastructure/
          │   ├── civo_api_key
          │   ├── digitalocean_token
          │   └── dreamhost_api_key
          └── database/
              └── neon/
                  └── connection_string
          |
          v (exposed as environment variables)
          |
nix/secrets/default.nix -> home.sessionVariables
          |
          v (read by prompt-pulse via getAPIKeyFromEnvOrFile())
          |
collectors/billing/collector.go
```

### Secret Lifecycle

1. **Encryption** (one-time setup):
   ```bash
   cd ~/git/crush-dots/nix/secrets
   # Edit secrets file
   sops common.yaml
   # Add new secret:
   # infrastructure:
   #   civo_api_key: your-api-key-here
   ```

2. **Home-Manager Activation** (after `home-manager switch`):
   ```bash
   # Decrypts common.yaml using age key
   # Writes decrypted secrets to ~/.config/sops-nix/secrets/
   # Sets file permissions to 0400 (read-only, owner only)
   ```

3. **Environment Variable Injection** (shell startup):
   ```bash
   # nix/secrets/default.nix sets:
   export CIVO_API_KEY_FILE="$HOME/.config/sops-nix/secrets/infrastructure/civo_api_key"
   ```

4. **Runtime Access** (prompt-pulse reads secrets):
   ```go
   // collectors/billing/collector.go:237
   func getAPIKeyFromEnvOrFile(envVar string) string {
       // 1. Check direct env var
       if key := os.Getenv(envVar); key != "" {
           return key
       }
       // 2. Check *_FILE variant
       fileEnv := envVar + "_FILE"
       if filePath := os.Getenv(fileEnv); filePath != "" {
           data, _ := os.ReadFile(filePath)
           return strings.TrimSpace(string(data))
       }
       return ""
   }
   ```

### Adding a New Secret

**Step 1**: Edit encrypted secrets file
```bash
cd ~/git/crush-dots/nix/secrets
sops common.yaml  # Opens in $EDITOR with decrypted content
```

**Step 2**: Add new secret entry
```yaml
# In sops editor
infrastructure:
  new_provider_key: "sk-new-provider-api-key-12345"
```

**Step 3**: Update secrets module (if needed)
```nix
# nix/secrets/default.nix
sops.secrets = {
  # ... existing secrets ...
  "infrastructure/new_provider_key" = { };
};

home.sessionVariables = {
  # ... existing vars ...
  NEW_PROVIDER_KEY_FILE = mkIf (hasSecret "infrastructure/new_provider_key")
    (secretPath "infrastructure/new_provider_key");
};
```

**Step 4**: Apply changes
```bash
cd ~/git/crush-dots
home-manager switch --flake .#jsullivan2@yoga
```

**Step 5**: Verify decryption
```bash
cat ~/.config/sops-nix/secrets/infrastructure/new_provider_key
# Output: sk-new-provider-api-key-12345
```

---

## Setup Checklist

### Prerequisites

- [ ] Age key generated (`~/.config/sops/age/keys.txt`)
- [ ] sops-nix configured in flake.nix inputs
- [ ] Home-manager activation successful

### Claude AI

- [x] OAuth credentials exist (`~/.claude/.credentials.json`)
- [x] Credentials file readable by prompt-pulse
- [ ] Optional: API key added to sops-nix for API accounts

### Cloud Billing

#### Civo
- [x] API key stored in sops-nix (`infrastructure/civo_api_key`)
- [x] Environment variable exposed (`CIVO_API_KEY_FILE`)
- [x] Collector can read secret file

#### DigitalOcean
- [x] API token stored in sops-nix (`infrastructure/digitalocean_token`)
- [x] Environment variable exposed (`DIGITALOCEAN_TOKEN_FILE`)
- [x] Collector can read secret file

#### DreamHost
- [x] API key stored in sops-nix (`infrastructure/dreamhost_api_key`)
- [x] Environment variable exposed (`DREAMHOST_API_KEY_FILE`)
- [x] Collector can read secret file

#### AWS
- [ ] AWS CLI installed (`sudo dnf install awscli2`)
- [ ] AWS CLI profile configured (`aws configure`)
- [ ] IAM permissions granted (Cost Explorer read-only)
- [ ] Test Cost Explorer access (`aws ce get-cost-and-usage --time-period ...`)

### Infrastructure Monitoring

#### Tailscale
- [x] Tailscale daemon running
- [x] `tailscale status --json` works (CLI fallback)
- [x] Optional: API key stored in sops-nix

#### Kubernetes
- [x] kubectl installed
- [x] Kubeconfig exists (`~/.kube/config`)
- [x] Contexts configured and accessible
- [x] Test cluster access (`kubectl get nodes`)

---

## Diagnostic Commands

### Check Secret Decryption

```bash
# List all decrypted secrets
ls -la ~/.config/sops-nix/secrets/api/
ls -la ~/.config/sops-nix/secrets/infrastructure/

# Read a specific secret (should show decrypted value)
cat ~/.config/sops-nix/secrets/infrastructure/civo_api_key

# Verify environment variables are set
env | grep -iE "_FILE" | sort
```

### Test Collectors

```bash
# Full status check (requires all secrets configured)
prompt-pulse status

# Check billing providers
prompt-pulse tui  # Press 'b' for billing view

# Check Claude accounts
prompt-pulse tui  # Press 'c' for Claude view

# Check infrastructure
prompt-pulse tui  # Press 'i' for infrastructure view
```

### Debug Missing Secrets

```bash
# Check which secrets are missing
prompt-pulse --diagnose

# Verbose logging
prompt-pulse --daemon --log-level debug

# Check systemd service logs
journalctl --user -u prompt-pulse -f
```

---

## Troubleshooting

### "API key not found in environment"

**Symptom**: Billing collector shows `auth_failed` status.

**Diagnosis**:
```bash
# Check if *_FILE env var is set
echo $CIVO_API_KEY_FILE

# Check if secret file exists and is readable
ls -la $CIVO_API_KEY_FILE
cat $CIVO_API_KEY_FILE
```

**Fix**:
- If env var is empty: Re-run `home-manager switch`
- If file doesn't exist: Check sops-nix activation errors
- If file is empty: Re-encrypt secrets with correct value

### "aws CLI not found"

**Symptom**: AWS billing shows `error` status with "aws CLI not found".

**Fix**:
```bash
# Install AWS CLI
sudo dnf install awscli2

# Verify installation
which aws
aws --version
```

### "kubectl: command not found"

**Symptom**: Kubernetes collector fails to run.

**Fix**:
```bash
# kubectl should be provided by Nix
which kubectl

# If missing, ensure base.nix includes k8sPackages
# Re-run home-manager switch
cd ~/git/crush-dots
home-manager switch --flake .#jsullivan2@yoga
```

### Secrets Not Decrypting

**Symptom**: `~/.config/sops-nix/secrets/` is empty or outdated.

**Diagnosis**:
```bash
# Check age key exists
ls -la ~/.config/sops/age/keys.txt

# Verify home-manager activation
home-manager generations | head -5

# Check for activation errors
journalctl --user -u home-manager-jsullivan2 -n 50
```

**Fix**:
```bash
# Force re-activation
home-manager switch --flake .#jsullivan2@yoga --impure

# If age key is missing, generate it:
mkdir -p ~/.config/sops/age
ssh-to-age -private-key < ~/.ssh/id_ed25519 > ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt
```

---

## Security Considerations

### File Permissions

All decrypted secrets have restrictive permissions:
```bash
# Secrets are readable only by owner
ls -la ~/.config/sops-nix/secrets/api/anthropic
# Output: -r-------- 1 jsullivan2 jsullivan2 108 Feb  5 07:14 ...
```

### Environment Variable Exposure

Secrets exposed via `*_FILE` environment variables are **paths**, not values:
```bash
# Safe - exposes file path
echo $CIVO_API_KEY_FILE
# Output: /home/jsullivan2/.config/sops-nix/secrets/infrastructure/civo_api_key

# Secret value only read when needed
cat $CIVO_API_KEY_FILE
```

### Git Safety

- ✅ Encrypted secrets (`common.yaml`) are safe to commit
- ❌ Age private keys (`~/.config/sops/age/keys.txt`) are gitignored
- ❌ Decrypted secrets (`~/.config/sops-nix/secrets/*`) are not in Git

### Systemd Service Isolation

The prompt-pulse daemon runs with security hardening:
```ini
# nix/home-manager/prompt-pulse.nix (systemd service)
NoNewPrivileges = true
ProtectSystem = "strict"
ProtectHome = "read-only"
ReadWritePaths = [
  "%h/.cache/prompt-pulse"
  "%h/.local/state/log"
]
```

---

## References

- **sops-nix Documentation**: https://github.com/Mic92/sops-nix
- **Age Encryption**: https://github.com/FiloSottile/age
- **Prompt-Pulse Config**: `/home/jsullivan2/.config/prompt-pulse/config.yaml`
- **Secrets Module**: `/home/jsullivan2/git/crush-dots/nix/secrets/default.nix`
- **Prompt-Pulse Module**: `/home/jsullivan2/git/crush-dots/nix/home-manager/prompt-pulse.nix`
- **Collector Source**: `/home/jsullivan2/git/crush-dots/cmd/prompt-pulse/collectors/`
