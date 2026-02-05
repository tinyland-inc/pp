# Prompt-Pulse Secrets Audit Summary

**Date**: 2026-02-05
**System**: yoga (Rocky Linux 10)
**Status**: ✅ **17/18 checks passed** (94% ready)

---

## Executive Summary

Prompt-pulse is **fully functional** for rich data ingestion with the following capabilities:

### ✅ Working (No Action Required)
- **Claude AI**: OAuth credentials configured, subscription account monitoring active
- **Civo Billing**: API key configured via sops-nix, billing monitoring active
- **DigitalOcean Billing**: Token configured via sops-nix, billing monitoring active
- **DreamHost Billing**: API key configured via sops-nix, billing monitoring active
- **Tailscale Mesh**: CLI fallback functional, mesh monitoring active
- **Kubernetes**: 2 contexts configured (DigitalOcean + Civo), cluster monitoring active

### ❌ Missing (Optional)
- **AWS Billing**: AWS CLI installed but credentials not configured
  - **Impact**: No AWS spend tracking or forecasting
  - **Cost Warning**: AWS Cost Explorer charges $0.01/API call (~$21/month at 15-minute intervals)
  - **Recommendation**: Only enable if AWS spend tracking is needed

---

## Detailed Inventory

### 1. Claude AI Collector

| Component | Status | Details |
|-----------|--------|---------|
| OAuth Credentials | ✅ Configured | `~/.claude/.credentials.json` (452 bytes) |
| Subscription Account | ✅ Active | Primary account monitored |
| API Key Accounts | ⚠️ Optional | Not configured (can add via sops-nix) |
| Rate Limit Tracking | ✅ Functional | Via OAuth refresh tokens |

**Test Command**:
```bash
prompt-pulse tui  # Press 'c' for Claude view
```

---

### 2. Billing Collector

#### Civo
| Component | Status | Details |
|-----------|--------|---------|
| API Key (sops-nix) | ✅ Configured | `infrastructure/civo_api_key` (24 bytes) |
| Environment Variable | ✅ Set | `CIVO_API_KEY_FILE` |
| Collector Status | ✅ Functional | Region: NYC1 |

#### DigitalOcean
| Component | Status | Details |
|-----------|--------|---------|
| Token (sops-nix) | ✅ Configured | `infrastructure/digitalocean_token` (30 bytes) |
| Environment Variable | ✅ Set | `DIGITALOCEAN_TOKEN_FILE` |
| Collector Status | ✅ Functional | Balance + invoices API |

#### DreamHost
| Component | Status | Details |
|-----------|--------|---------|
| API Key (sops-nix) | ✅ Configured | `infrastructure/dreamhost_api_key` (29 bytes) |
| Environment Variable | ✅ Set | `DREAMHOST_API_KEY_FILE` |
| Collector Status | ✅ Functional | Bandwidth + credits tracking |

#### AWS
| Component | Status | Details |
|-----------|--------|---------|
| AWS CLI | ✅ Installed | `/home/jsullivan2/.nix-profile/bin/aws` |
| Credentials | ❌ Not configured | Run `aws configure` to enable |
| Collector Status | ⚠️ Disabled | Will show "auth_failed" until credentials added |

**Test Command**:
```bash
prompt-pulse tui  # Press 'b' for billing view
```

---

### 3. Infrastructure Collector

#### Tailscale
| Component | Status | Details |
|-----------|--------|---------|
| CLI | ✅ Installed | `/usr/bin/tailscale` |
| Daemon Status | ✅ Running | Tailnet: `taila4c78d.ts.net` |
| API Key (sops-nix) | ⚠️ Optional | Configured but unused (CLI fallback works) |
| Collector Status | ✅ Functional | Via `tailscale status --json` |

#### Kubernetes
| Component | Status | Details |
|-----------|--------|---------|
| kubectl | ✅ Installed | Via Nix (1.31.2) |
| Kubeconfig | ✅ Present | `~/.kube/config` |
| Contexts | ✅ 2 configured | `do-nyc2-...`, `tinyland-civo-dev` |
| Collector Status | ✅ Functional | Node + pod metrics |

**Test Command**:
```bash
prompt-pulse tui  # Press 'i' for infrastructure view
```

---

## Secret Management Architecture

### sops-nix Integration

```
┌─────────────────────────────────────────────────────────────────┐
│ crush-dots/nix/secrets/common.yaml (encrypted with age)         │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           │ home-manager switch (decrypts)
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│ ~/.config/sops-nix/secrets/                                     │
│   ├── api/                                                      │
│   │   ├── anthropic        (mode 0400, owner: jsullivan2)      │
│   │   └── perplexity                                           │
│   ├── infrastructure/                                           │
│   │   ├── civo_api_key                                         │
│   │   ├── digitalocean_token                                   │
│   │   └── dreamhost_api_key                                    │
│   └── database/                                                 │
│       └── neon/connection_string                               │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           │ nix/secrets/default.nix (exports env vars)
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│ Environment Variables (shell session)                           │
│   CIVO_API_KEY_FILE=/home/jsullivan2/.config/sops-nix/...      │
│   DIGITALOCEAN_TOKEN_FILE=...                                   │
│   DREAMHOST_API_KEY_FILE=...                                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           │ collectors/billing/collector.go:237
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│ getAPIKeyFromEnvOrFile()                                        │
│   1. Check direct env var (e.g., CIVO_API_KEY)                 │
│   2. Check *_FILE variant (e.g., CIVO_API_KEY_FILE)            │
│   3. Read file if *_FILE exists                                │
│   4. Return decrypted secret value                             │
└─────────────────────────────────────────────────────────────────┘
```

### Key Security Features

1. **Encrypted at Rest**: All secrets in `common.yaml` are encrypted with age
2. **Decrypted on Activation**: Secrets only decrypted during `home-manager switch`
3. **Restrictive Permissions**: Decrypted secrets are mode 0400 (read-only, owner only)
4. **File-based Access**: Environment variables store paths, not secret values
5. **Runtime Reading**: Prompt-pulse reads secrets from files at runtime
6. **Systemd Isolation**: Daemon runs with `ProtectHome=read-only` and resource limits

---

## Configuration File

Current configuration: `~/.config/prompt-pulse/config.yaml`

```yaml
accounts:
  claude:
    - name: "primary"
      type: "subscription"
      credentials_path: "/home/jsullivan2/.claude/.credentials.json"
      enabled: true
  civo:
    api_key_env: "CIVO_API_KEY"
    region: "NYC1"
  digitalocean:
    api_key_env: "DIGITALOCEAN_TOKEN"
  dreamhost:
    api_key_env: "DREAMHOST_API_KEY"
  aws:
    profile: "default"
    regions: ["us-east-1"]

tailscale:
  tailnet: "tinyland.ts.net"
  api_key_env: "TAILSCALE_API_KEY"
  use_cli_fallback: true

kubernetes:
  contexts:
    - name: "tinyland-civo-dev"
      kubeconfig: ""
      namespace: "fuzzy-dev"
      dashboard_url: "https://dashboard.civo.com"
```

---

## Diagnostic Script

A comprehensive diagnostic script is available:

```bash
# Run full diagnostic
~/git/crush-dots/cmd/prompt-pulse/scripts/check-secrets.sh

# Output includes:
# - Core configuration checks
# - Claude AI secrets
# - Cloud billing secrets (Civo, DO, DreamHost, AWS)
# - Infrastructure monitoring (Tailscale, Kubernetes)
# - Environment variables summary
# - Prompt-pulse daemon status
```

**Current Result**: 17/18 checks passed (only AWS credentials missing)

---

## How to Enable AWS Billing (Optional)

If AWS spend tracking is desired:

```bash
# 1. Configure AWS CLI credentials
aws configure
# Enter:
#   AWS Access Key ID: [from AWS IAM]
#   AWS Secret Access Key: [from AWS IAM]
#   Default region: us-east-1

# 2. Grant IAM permissions
# Attach policy with:
#   - ce:GetCostAndUsage
#   - ce:GetCostForecast

# 3. Test Cost Explorer access
aws ce get-cost-and-usage \
  --time-period Start=2026-02-01,End=2026-02-05 \
  --granularity DAILY \
  --metrics UnblendedCost

# 4. Verify in prompt-pulse
prompt-pulse tui  # Press 'b' for billing view
```

**Cost Warning**: AWS Cost Explorer charges **$0.01 per API call**. At prompt-pulse's default 15-minute poll interval, this costs approximately:
- **$0.72/day** (3 calls/hour × 24 hours × $0.01)
- **$21.60/month** (30 days × $0.72)

**Recommendation**: Increase poll interval to 1 hour to reduce costs to ~$5.40/month.

---

## Adding New Secrets

### Step-by-Step Process

1. **Edit encrypted secrets file**:
   ```bash
   cd ~/git/crush-dots/nix/secrets
   sops common.yaml  # Opens in $EDITOR with decrypted content
   ```

2. **Add new secret entry**:
   ```yaml
   # In sops editor
   infrastructure:
     new_provider_api_key: "sk-new-provider-key-12345"
   ```

3. **Update secrets module** (if needed):
   ```nix
   # nix/secrets/default.nix
   sops.secrets = {
     # ... existing secrets ...
     "infrastructure/new_provider_api_key" = { };
   };

   home.sessionVariables = {
     # ... existing vars ...
     NEW_PROVIDER_API_KEY_FILE = mkIf (hasSecret "infrastructure/new_provider_api_key")
       (secretPath "infrastructure/new_provider_api_key");
   };
   ```

4. **Apply changes**:
   ```bash
   cd ~/git/crush-dots
   home-manager switch --flake .#jsullivan2@yoga
   ```

5. **Verify decryption**:
   ```bash
   cat ~/.config/sops-nix/secrets/infrastructure/new_provider_api_key
   echo $NEW_PROVIDER_API_KEY_FILE
   ```

---

## Troubleshooting Quick Reference

### Secrets Not Decrypting
```bash
# Check age key exists
ls -la ~/.config/sops/age/keys.txt

# Re-run home-manager switch
cd ~/git/crush-dots
home-manager switch --flake .#jsullivan2@yoga

# Check for errors
journalctl --user -u home-manager-jsullivan2 -n 50
```

### Environment Variables Not Set
```bash
# Reload shell environment
source ~/.bashrc

# Check variables are exported
env | grep -iE "_FILE" | sort

# Verify home-manager activation
home-manager generations | head -5
```

### Collector Shows "auth_failed"
```bash
# Check secret file exists and is readable
ls -la ~/.config/sops-nix/secrets/infrastructure/civo_api_key
cat ~/.config/sops-nix/secrets/infrastructure/civo_api_key

# Check environment variable is set
echo $CIVO_API_KEY_FILE

# Restart daemon to pick up new environment
systemctl --user restart prompt-pulse
```

---

## Files Referenced

| File | Purpose |
|------|---------|
| `/home/jsullivan2/.config/prompt-pulse/config.yaml` | Main configuration (generated by Nix) |
| `/home/jsullivan2/.claude/.credentials.json` | Claude OAuth credentials |
| `/home/jsullivan2/.config/sops-nix/secrets/` | Decrypted secrets directory |
| `/home/jsullivan2/.config/sops/age/keys.txt` | Age private key for decryption |
| `/home/jsullivan2/git/crush-dots/nix/secrets/common.yaml` | Encrypted secrets source |
| `/home/jsullivan2/git/crush-dots/nix/secrets/default.nix` | sops-nix module |
| `/home/jsullivan2/git/crush-dots/nix/home-manager/prompt-pulse.nix` | Prompt-pulse Nix module |
| `/home/jsullivan2/git/crush-dots/cmd/prompt-pulse/collectors/billing/collector.go` | Billing collector implementation |

---

## Service Status

```bash
# Current status
systemctl --user status prompt-pulse

# Daemon uptime: 2026-02-05 05:58:38
# Log file: /home/jsullivan2/.local/state/log/prompt-pulse.log (684K)
# Status: ✅ Running
```

---

## Next Steps

### Immediate (No Action Required)
- ✅ All critical secrets configured
- ✅ All collectors functional (except AWS)
- ✅ Daemon running and healthy

### Optional Enhancements

1. **Add AWS Billing** (if needed):
   - Follow AWS setup guide above
   - Consider increasing poll interval to reduce costs

2. **Add Claude API Key Accounts** (if needed):
   - Add `api/anthropic` to sops-nix
   - Configure API account in `config.yaml`

3. **Add More Kubernetes Contexts**:
   - Add kubeconfig entries
   - Update `config.yaml` with new contexts

---

## Documentation

- **Full Setup Guide**: `docs/SECRETS_SETUP.md`
- **Quick Reference**: `docs/SECRETS_QUICK_REFERENCE.md`
- **This Summary**: `docs/SECRETS_SUMMARY.md`
- **Diagnostic Script**: `scripts/check-secrets.sh`

---

## Conclusion

Prompt-pulse is **production-ready** on yoga with:
- ✅ 17/18 checks passed
- ✅ All critical data sources configured
- ✅ Secure secrets management via sops-nix
- ✅ Background daemon running with resource limits
- ✅ Interactive TUI and Starship integration functional

**Only missing component**: AWS billing (optional, expensive, easy to add if needed)
