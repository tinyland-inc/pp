# Prompt-Pulse Secrets Documentation Index

**Last Updated**: 2026-02-05

This index organizes all secrets-related documentation for prompt-pulse rich data ingestion.

---

## 📚 Documentation Hierarchy

```
docs/
├── SECRETS_INDEX.md              ← You are here (navigation hub)
├── SECRETS_SUMMARY.md            ← Executive summary + current status
├── SECRETS_QUICK_REFERENCE.md    ← Common commands and one-liners
└── SECRETS_SETUP.md              ← Comprehensive setup guide

scripts/
└── check-secrets.sh              ← Diagnostic tool
```

---

## 🚀 Quick Start (New Users)

**First time setting up secrets?** Follow this path:

1. **[Run Diagnostic](../scripts/check-secrets.sh)** (30 seconds)
   ```bash
   ~/git/crush-dots/cmd/prompt-pulse/scripts/check-secrets.sh
   ```

2. **[Read Summary](SECRETS_SUMMARY.md)** (5 minutes)
   - Current status on yoga
   - What's working vs. what's missing
   - Architecture overview

3. **[Quick Reference](SECRETS_QUICK_REFERENCE.md)** (2 minutes)
   - Essential commands
   - Common tasks
   - Troubleshooting one-liners

4. **[Setup Guide](SECRETS_SETUP.md)** (15 minutes, as needed)
   - Complete setup instructions
   - Detailed configuration
   - All collectors documented

---

## 📖 Document Descriptions

### [SECRETS_SUMMARY.md](SECRETS_SUMMARY.md)
**Purpose**: Executive summary of current secrets configuration

**What's Inside**:
- Current status (17/18 checks passed)
- Detailed inventory by collector
- Architecture diagram
- Configuration file reference
- Service status

**When to Read**: First thing, or anytime you want a status overview

**Read Time**: 5 minutes

---

### [SECRETS_QUICK_REFERENCE.md](SECRETS_QUICK_REFERENCE.md)
**Purpose**: Quick command reference for daily operations

**What's Inside**:
- Current status table
- Essential commands (check status, add secrets, fix issues)
- Secrets inventory (required vs. optional)
- Environment variable patterns
- Troubleshooting one-liners
- Quick diagnostic script

**When to Read**: When you need to perform a specific task quickly

**Read Time**: 2 minutes

---

### [SECRETS_SETUP.md](SECRETS_SETUP.md)
**Purpose**: Comprehensive setup guide with all details

**What's Inside**:
- Complete secrets inventory (all collectors)
- Current configuration status (per-secret breakdown)
- How sops-nix works (architecture + lifecycle)
- Adding new secrets (step-by-step)
- Setup checklist (all requirements)
- Diagnostic commands
- Troubleshooting (common issues + fixes)
- Security considerations

**When to Read**: When setting up from scratch, or adding new collectors

**Read Time**: 15 minutes

---

## 🔧 Diagnostic Script

### [check-secrets.sh](../scripts/check-secrets.sh)
**Purpose**: Automated comprehensive diagnostic tool

**What It Checks**:
1. Core configuration (config.yaml, age key, sops, age CLI)
2. Claude AI secrets (OAuth credentials, API keys)
3. Cloud billing secrets (Civo, DigitalOcean, DreamHost, AWS)
4. Infrastructure monitoring (Tailscale, Kubernetes)
5. Environment variables summary
6. Prompt-pulse service status

**Output**: Color-coded report with ✅/❌/⚠️ status indicators

**Run Time**: 30 seconds

**Usage**:
```bash
~/git/crush-dots/cmd/prompt-pulse/scripts/check-secrets.sh
```

---

## 🎯 Use Cases

### "I'm setting up prompt-pulse for the first time"
1. **[Run Diagnostic](../scripts/check-secrets.sh)** - See what's missing
2. **[Setup Guide](SECRETS_SETUP.md)** - Follow complete setup instructions
3. **[Run Diagnostic](../scripts/check-secrets.sh)** - Verify everything works

---

### "Something stopped working"
1. **[Quick Reference](SECRETS_QUICK_REFERENCE.md)** - Check troubleshooting section
2. **[Run Diagnostic](../scripts/check-secrets.sh)** - Identify the problem
3. **[Setup Guide](SECRETS_SETUP.md)** - Find detailed fix instructions

---

### "I want to add a new cloud provider"
1. **[Setup Guide](SECRETS_SETUP.md)** - See "Adding New Secrets" section
2. **[Quick Reference](SECRETS_QUICK_REFERENCE.md)** - Use command templates
3. **[Run Diagnostic](../scripts/check-secrets.sh)** - Verify new secret works

---

### "What's my current status?"
1. **[Run Diagnostic](../scripts/check-secrets.sh)** - Get instant report
2. **[Summary](SECRETS_SUMMARY.md)** - Read detailed status

---

### "I just want the quick commands"
1. **[Quick Reference](SECRETS_QUICK_REFERENCE.md)** - Go straight here

---

## 🔐 Secrets by Collector

Quick reference for which document covers each collector:

| Collector | Summary | Quick Ref | Setup Guide |
|-----------|---------|-----------|-------------|
| **Claude AI** | Status + OAuth | ✅ | ✅ Full details |
| **Civo Billing** | Status | ✅ One-liner | ✅ API key setup |
| **DigitalOcean** | Status | ✅ One-liner | ✅ Token setup |
| **DreamHost** | Status | ✅ One-liner | ✅ API key setup |
| **AWS Billing** | Status + setup | ✅ Commands | ✅ Full guide + cost warning |
| **Tailscale** | Status | ✅ CLI fallback | ✅ API vs. CLI |
| **Kubernetes** | Status | ✅ Context check | ✅ Kubeconfig setup |

---

## 📋 Current Status (yoga)

| Category | Status | Next Steps |
|----------|--------|------------|
| **Core Config** | ✅ 4/4 checks | None |
| **Claude** | ✅ 2/2 checks | Optional: Add API key accounts |
| **Billing** | ⚠️ 11/12 checks | Optional: Configure AWS ($21/month) |
| **Infrastructure** | ✅ 6/6 checks | None |
| **Overall** | ✅ 17/18 (94%) | Production-ready |

**Only Missing**: AWS credentials (optional, expensive)

---

## 🛠️ Common Tasks by Document

### Check Status
- **Quick Command**: [Quick Reference → Check Secrets Status](SECRETS_QUICK_REFERENCE.md#check-current-status)
- **Full Report**: [Run Diagnostic Script](../scripts/check-secrets.sh)
- **Detailed Status**: [Summary → Detailed Inventory](SECRETS_SUMMARY.md#detailed-inventory)

### Add New Secret
- **Quick Steps**: [Quick Reference → Add a New Secret](SECRETS_QUICK_REFERENCE.md#add-a-new-secret)
- **Full Guide**: [Setup Guide → Adding New Secrets](SECRETS_SETUP.md#adding-a-new-secret)

### Fix Broken Secret
- **Quick Fix**: [Quick Reference → Fix Missing Secrets](SECRETS_QUICK_REFERENCE.md#fix-missing-secrets)
- **Detailed Troubleshooting**: [Setup Guide → Troubleshooting](SECRETS_SETUP.md#troubleshooting)

### Enable AWS
- **Quick Steps**: [Quick Reference → Missing: AWS Setup](SECRETS_QUICK_REFERENCE.md#missing-aws-setup)
- **Full Guide**: [Summary → How to Enable AWS Billing](SECRETS_SUMMARY.md#how-to-enable-aws-billing-optional)
- **Cost Analysis**: [Setup Guide → AWS Configuration](SECRETS_SETUP.md#aws-configuration)

### Understand Architecture
- **Quick Diagram**: [Quick Reference → How sops-nix Works](SECRETS_QUICK_REFERENCE.md) (not included, go to Summary)
- **Full Architecture**: [Summary → Secret Management Architecture](SECRETS_SUMMARY.md#secret-management-architecture)
- **Detailed Lifecycle**: [Setup Guide → How sops-nix Works](SECRETS_SETUP.md#how-sops-nix-works)

---

## 🔍 Finding Information

### By Topic

| Topic | Document | Section |
|-------|----------|---------|
| **sops-nix architecture** | Setup Guide | How sops-nix Works |
| **Environment variables** | Summary | Architecture diagram |
| **Current status** | Summary | Executive Summary |
| **AWS cost warning** | All docs | AWS sections |
| **Security** | Setup Guide | Security Considerations |
| **File locations** | Summary | Files Referenced |
| **Troubleshooting** | Setup Guide + Quick Ref | Troubleshooting sections |

### By Question

| Question | Answer Location |
|----------|----------------|
| "What's working?" | [Summary → Executive Summary](SECRETS_SUMMARY.md#executive-summary) |
| "What's missing?" | [Run Diagnostic](../scripts/check-secrets.sh) |
| "How do I add a secret?" | [Setup Guide → Adding New Secrets](SECRETS_SETUP.md#adding-a-new-secret) |
| "Why use sops-nix?" | [Summary → Secret Management Architecture](SECRETS_SUMMARY.md#secret-management-architecture) |
| "Where are my secrets?" | [Summary → Files Referenced](SECRETS_SUMMARY.md#files-referenced) |
| "How much does AWS cost?" | [Quick Ref → Missing: AWS Setup](SECRETS_QUICK_REFERENCE.md#missing-aws-setup) |
| "Is this secure?" | [Setup Guide → Security Considerations](SECRETS_SETUP.md#security-considerations) |

---

## 📱 Mobile-Friendly Quick Reference

For quick access on mobile or when SSH'd in:

```bash
# Bookmark these commands
alias pp-check='~/git/crush-dots/cmd/prompt-pulse/scripts/check-secrets.sh'
alias pp-docs='cat ~/git/crush-dots/cmd/prompt-pulse/docs/SECRETS_SUMMARY.md | less'
alias pp-help='cat ~/git/crush-dots/cmd/prompt-pulse/docs/SECRETS_QUICK_REFERENCE.md | less'
```

---

## 🔗 External Links

### API Key Locations
- **Anthropic**: https://console.anthropic.com/settings/keys
- **Civo**: https://dashboard.civo.com/security
- **DigitalOcean**: https://cloud.digitalocean.com/account/api/tokens
- **DreamHost**: https://panel.dreamhost.com/?tree=home.api
- **Tailscale**: https://login.tailscale.com/admin/settings/keys

### Documentation
- **sops-nix**: https://github.com/Mic92/sops-nix
- **age Encryption**: https://github.com/FiloSottile/age
- **Anthropic API**: https://docs.anthropic.com/api
- **AWS Cost Explorer**: https://docs.aws.amazon.com/cost-management/latest/userguide/ce-api.html

---

## 📝 Document Stats

| Document | Length | Read Time | Last Updated |
|----------|--------|-----------|--------------|
| SECRETS_SUMMARY.md | ~700 lines | 5 min | 2026-02-05 |
| SECRETS_QUICK_REFERENCE.md | ~300 lines | 2 min | 2026-02-05 |
| SECRETS_SETUP.md | ~900 lines | 15 min | 2026-02-05 |
| check-secrets.sh | ~350 lines | 30 sec (run) | 2026-02-05 |

**Total Documentation**: ~2,250 lines covering all aspects of secrets management

---

## 🎓 Learning Path

### Beginner (Never used prompt-pulse)
1. [Summary](SECRETS_SUMMARY.md) - Understand what you're getting into
2. [Run Diagnostic](../scripts/check-secrets.sh) - See current state
3. [Setup Guide](SECRETS_SETUP.md) - Follow step-by-step

### Intermediate (Have prompt-pulse running)
1. [Run Diagnostic](../scripts/check-secrets.sh) - Check everything still works
2. [Quick Reference](SECRETS_QUICK_REFERENCE.md) - Learn maintenance commands
3. [Setup Guide](SECRETS_SETUP.md) - Reference as needed

### Advanced (Adding new collectors)
1. [Setup Guide → Adding New Secrets](SECRETS_SETUP.md#adding-a-new-secret)
2. [Summary → Architecture](SECRETS_SUMMARY.md#secret-management-architecture)
3. [Modify collector code](../collectors/)

---

## 🆘 Support

**Something not working?**
1. [Run Diagnostic](../scripts/check-secrets.sh) - Get detailed report
2. [Quick Reference → Troubleshooting](SECRETS_QUICK_REFERENCE.md#troubleshooting-one-liners)
3. [Setup Guide → Troubleshooting](SECRETS_SETUP.md#troubleshooting)

**Still stuck?**
- Check daemon logs: `journalctl --user -u prompt-pulse -f`
- Review configuration: `cat ~/.config/prompt-pulse/config.yaml`
- Verify secrets decrypted: `ls -la ~/.config/sops-nix/secrets/`

---

## ✅ Success Criteria

You know secrets are working when:
- ✅ Diagnostic script shows 17/18+ checks passed
- ✅ `prompt-pulse tui` displays data from all collectors
- ✅ Starship prompt shows Claude/billing/infra modules
- ✅ No "auth_failed" errors in TUI
- ✅ Daemon runs without errors (`systemctl --user status prompt-pulse`)

---

**Last Updated**: 2026-02-05
**Current Status**: Production-ready (17/18 checks passed on yoga)
