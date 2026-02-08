# Prompt-Pulse Secrets Quick Reference

**Quick Start**: All secrets needed for full functionality

---

## Current Status (yoga)

| Category | Status | Details |
|----------|--------|---------|
| **Claude** | ✅ Working | OAuth credentials configured |
| **Civo Billing** | ✅ Working | API key via sops-nix |
| **DigitalOcean Billing** | ✅ Working | Token via sops-nix |
| **DreamHost Billing** | ✅ Working | API key via sops-nix |
| **AWS Billing** | ❌ Missing | AWS CLI not installed |
| **Tailscale Mesh** | ✅ Working | CLI fallback (no API key needed) |
| **Kubernetes** | ✅ Working | 2 contexts configured |

---

## Essential Commands

### Check Secrets Status
```bash
# List all decrypted secrets
ls -la ~/.config/sops-nix/secrets/api/
ls -la ~/.config/sops-nix/secrets/infrastructure/

# Verify environment variables
env | grep -iE "(API_KEY|TOKEN)_FILE" | sort

# Test prompt-pulse
prompt-pulse status
prompt-pulse tui
```

### Add a New Secret
```bash
# 1. Edit encrypted secrets file
cd ~/git/crush-dots/nix/secrets
sops common.yaml

# 2. Add entry (e.g., for AWS)
# infrastructure:
#   aws_access_key: "AKIA..."
#   aws_secret_key: "..."

# 3. Update secrets module (if needed)
# Edit: nix/secrets/default.nix
# Add: sops.secrets."infrastructure/aws_access_key" = { };

# 4. Apply changes
cd ~/git/crush-dots
home-manager switch --flake .#jsullivan2@yoga

# 5. Verify
cat ~/.config/sops-nix/secrets/infrastructure/aws_access_key
```

### Fix Missing Secrets
```bash
# Re-activate home-manager (re-decrypts secrets)
home-manager switch --flake .#jsullivan2@yoga

# Check for errors
journalctl --user -u home-manager-jsullivan2 -n 50

# Verify age key exists
ls -la ~/.config/sops/age/keys.txt
```

---

## Secrets Inventory

### Required (for full functionality)

| Secret | Purpose | How to Get | Status |
|--------|---------|------------|--------|
| `~/.claude/.credentials.json` | Claude OAuth | `claude login` | ✅ |
| `infrastructure/civo_api_key` | Civo billing | https://dashboard.civo.com/security | ✅ |
| `infrastructure/digitalocean_token` | DO billing | https://cloud.digitalocean.com/account/api/tokens | ✅ |
| AWS CLI profile | AWS billing | `aws configure` | ❌ |

### Optional (for extended monitoring)

| Secret | Purpose | Status |
|--------|---------|--------|
| `infrastructure/dreamhost_api_key` | DreamHost billing | ✅ |
| `infrastructure/tailscale_auth_key` | Tailscale API (CLI fallback works) | ⚠️ Unused |
| `api/anthropic` | Claude API key accounts | ⚠️ Optional |

---

## Environment Variable Patterns

All secrets follow the `*_FILE` pattern:

```bash
# Direct env var (optional)
export CIVO_API_KEY="..."

# File-based (preferred, via sops-nix)
export CIVO_API_KEY_FILE="~/.config/sops-nix/secrets/infrastructure/civo_api_key"

# Prompt-pulse reads from *_FILE if available, falls back to direct env var
```

**Supported Patterns**:
- `CIVO_API_KEY` / `CIVO_API_KEY_FILE`
- `DIGITALOCEAN_TOKEN` / `DIGITALOCEAN_TOKEN_FILE`
- `DREAMHOST_API_KEY` / `DREAMHOST_API_KEY_FILE`
- `TAILSCALE_API_KEY` / `TAILSCALE_API_KEY_FILE`
- `ANTHROPIC_API_KEY` (direct only, no *_FILE support for API accounts)

---

## Missing: AWS Setup

To enable AWS billing monitoring:

```bash
# 1. Install AWS CLI
sudo dnf install awscli2

# 2. Configure credentials
aws configure
# AWS Access Key ID: (from IAM)
# AWS Secret Access Key: (from IAM)
# Default region: us-east-1

# 3. Grant Cost Explorer permissions
# In AWS IAM, attach policy with:
#   - ce:GetCostAndUsage
#   - ce:GetCostForecast

# 4. Test
aws ce get-cost-and-usage \
  --time-period Start=2026-02-01,End=2026-02-05 \
  --granularity DAILY \
  --metrics UnblendedCost

# 5. Enable in config
# ~/.config/prompt-pulse/config.yaml
# accounts:
#   aws:
#     profile: "default"
#     regions: ["us-east-1"]
```

**Cost Warning**: AWS Cost Explorer charges $0.01/API call. At 15-minute intervals, this costs ~$0.72/day ($21/month).

---

## Troubleshooting One-Liners

```bash
# Missing secrets?
ls -la ~/.config/sops-nix/secrets/*/ && echo "Secrets decrypted" || echo "Missing secrets"

# Age key missing?
ls -la ~/.config/sops/age/keys.txt || ssh-to-age -private-key < ~/.ssh/id_ed25519 > ~/.config/sops/age/keys.txt

# Environment variables not set?
source ~/.bashrc && env | grep -iE "_FILE"

# Prompt-pulse daemon not running?
systemctl --user status prompt-pulse

# Restart daemon
systemctl --user restart prompt-pulse

# View daemon logs
journalctl --user -u prompt-pulse -f
```

---

## Quick Diagnostic

Run this to check all secrets:

```bash
#!/bin/bash
echo "=== Secrets Diagnostic ==="
echo ""

# Claude OAuth
if [ -f ~/.claude/.credentials.json ]; then
  echo "✅ Claude OAuth: $(wc -l < ~/.claude/.credentials.json) lines"
else
  echo "❌ Claude OAuth: Missing"
fi

# Civo
if [ -f ~/.config/sops-nix/secrets/infrastructure/civo_api_key ]; then
  echo "✅ Civo API Key: $(wc -c < ~/.config/sops-nix/secrets/infrastructure/civo_api_key) bytes"
else
  echo "❌ Civo API Key: Missing"
fi

# DigitalOcean
if [ -f ~/.config/sops-nix/secrets/infrastructure/digitalocean_token ]; then
  echo "✅ DigitalOcean Token: $(wc -c < ~/.config/sops-nix/secrets/infrastructure/digitalocean_token) bytes"
else
  echo "❌ DigitalOcean Token: Missing"
fi

# DreamHost
if [ -f ~/.config/sops-nix/secrets/infrastructure/dreamhost_api_key ]; then
  echo "✅ DreamHost API Key: $(wc -c < ~/.config/sops-nix/secrets/infrastructure/dreamhost_api_key) bytes"
else
  echo "❌ DreamHost API Key: Missing"
fi

# AWS CLI
if command -v aws &>/dev/null; then
  echo "✅ AWS CLI: Installed"
  aws sts get-caller-identity &>/dev/null && echo "✅ AWS Credentials: Valid" || echo "⚠️  AWS Credentials: Not configured"
else
  echo "❌ AWS CLI: Not installed"
fi

# Tailscale
if command -v tailscale &>/dev/null; then
  echo "✅ Tailscale CLI: Installed"
  tailscale status &>/dev/null && echo "✅ Tailscale: Running" || echo "⚠️  Tailscale: Not running"
else
  echo "❌ Tailscale: Not installed"
fi

# Kubernetes
if command -v kubectl &>/dev/null; then
  echo "✅ kubectl: Installed"
  contexts=$(kubectl config get-contexts -o name 2>/dev/null | wc -l)
  echo "✅ Kubernetes Contexts: $contexts configured"
else
  echo "❌ kubectl: Not installed"
fi

echo ""
echo "=== Environment Variables ==="
env | grep -iE "(CIVO|DIGITALOCEAN|DREAMHOST|TAILSCALE|ANTHROPIC)_(API_KEY|TOKEN)_FILE" | sort
```

Save as `~/bin/check-prompt-pulse-secrets` and run anytime.

---

## See Also

- **Full Setup Guide**: `docs/SECRETS_SETUP.md`
- **Prompt-Pulse Config**: `~/.config/prompt-pulse/config.yaml`
- **Secrets Module**: `~/git/crush-dots/nix/secrets/default.nix`
- **sops-nix Docs**: https://github.com/Mic92/sops-nix
