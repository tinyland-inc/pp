#!/usr/bin/env bash
# Prompt-Pulse Secrets Diagnostic Script
# Checks all required secrets and configurations

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Icons
CHECK="✅"
CROSS="❌"
WARN="⚠️ "

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Prompt-Pulse Secrets Diagnostic"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

total_checks=0
passed_checks=0
failed_checks=0

check_file() {
  local name=$1
  local path=$2
  local required=${3:-false}

  total_checks=$((total_checks + 1))

  if [ -f "$path" ]; then
    local size
    size=$(wc -c < "$path")
    if [ "$size" -gt 0 ]; then
      echo -e "${GREEN}${CHECK}${NC} $name: $(wc -c < "$path") bytes"
      passed_checks=$((passed_checks + 1))
      return 0
    else
      echo -e "${RED}${CROSS}${NC} $name: Empty file"
      failed_checks=$((failed_checks + 1))
      return 1
    fi
  else
    if [ "$required" = "true" ]; then
      echo -e "${RED}${CROSS}${NC} $name: Missing"
      failed_checks=$((failed_checks + 1))
    else
      echo -e "${YELLOW}${WARN}${NC}$name: Not configured (optional)"
    fi
    return 1
  fi
}

check_command() {
  local name=$1
  local cmd=$2
  local required=${3:-false}

  total_checks=$((total_checks + 1))

  if command -v "$cmd" &>/dev/null; then
    echo -e "${GREEN}${CHECK}${NC} $name: Installed ($(command -v "$cmd"))"
    passed_checks=$((passed_checks + 1))
    return 0
  else
    if [ "$required" = "true" ]; then
      echo -e "${RED}${CROSS}${NC} $name: Not installed"
      failed_checks=$((failed_checks + 1))
    else
      echo -e "${YELLOW}${WARN}${NC}$name: Not installed (optional)"
    fi
    return 1
  fi
}

check_env_var() {
  local name=$1
  local var=$2

  if [ -n "${!var:-}" ]; then
    echo -e "${GREEN}${CHECK}${NC} $name: Set (${!var})"
    return 0
  else
    echo -e "${YELLOW}${WARN}${NC}$name: Not set"
    return 1
  fi
}

# ============================================================================
# 1. Core Configuration
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 1. Core Configuration                                                 ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

check_file "Prompt-Pulse Config" "$HOME/.config/prompt-pulse/config.yaml" true
check_file "Age Private Key" "$HOME/.config/sops/age/keys.txt" true
check_command "sops" "sops" true
check_command "age" "age" true

echo ""

# ============================================================================
# 2. Claude AI Secrets
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 2. Claude AI Secrets                                                  ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

check_file "Claude OAuth Credentials" "$HOME/.claude/.credentials.json" true

if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
  echo -e "${GREEN}${CHECK}${NC} Anthropic API Key: Set via environment variable"
  passed_checks=$((passed_checks + 1))
elif [ -f "$HOME/.config/sops-nix/secrets/api/anthropic" ]; then
  echo -e "${GREEN}${CHECK}${NC} Anthropic API Key: Available via sops-nix"
  passed_checks=$((passed_checks + 1))
else
  echo -e "${YELLOW}${WARN}${NC}Anthropic API Key: Not configured (optional for API accounts)"
fi

echo ""

# ============================================================================
# 3. Cloud Billing Secrets
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 3. Cloud Billing Secrets                                              ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

echo "Civo:"
check_file "  API Key (sops-nix)" "$HOME/.config/sops-nix/secrets/infrastructure/civo_api_key" true
check_env_var "  Env Var (CIVO_API_KEY_FILE)" "CIVO_API_KEY_FILE"

echo ""
echo "DigitalOcean:"
check_file "  Token (sops-nix)" "$HOME/.config/sops-nix/secrets/infrastructure/digitalocean_token" true
check_env_var "  Env Var (DIGITALOCEAN_TOKEN_FILE)" "DIGITALOCEAN_TOKEN_FILE"

echo ""
echo "DreamHost:"
check_file "  API Key (sops-nix)" "$HOME/.config/sops-nix/secrets/infrastructure/dreamhost_api_key" false
check_env_var "  Env Var (DREAMHOST_API_KEY_FILE)" "DREAMHOST_API_KEY_FILE"

echo ""
echo "AWS:"
if check_command "  AWS CLI" "aws" false; then
  if aws sts get-caller-identity &>/dev/null; then
    echo -e "${GREEN}${CHECK}${NC}   Credentials: Valid"
    identity=$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || echo "unknown")
    echo -e "      Account: $identity"
    passed_checks=$((passed_checks + 1))
  else
    echo -e "${RED}${CROSS}${NC}   Credentials: Not configured or invalid"
    echo -e "      Run: ${YELLOW}aws configure${NC}"
    failed_checks=$((failed_checks + 1))
  fi
else
  echo -e "${YELLOW}${WARN}${NC}  Setup: Run ${YELLOW}sudo dnf install awscli2 && aws configure${NC}"
fi

echo ""

# ============================================================================
# 4. Infrastructure Monitoring
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 4. Infrastructure Monitoring                                          ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

echo "Tailscale:"
if check_command "  CLI" "tailscale" true; then
  if tailscale status &>/dev/null; then
    echo -e "${GREEN}${CHECK}${NC}   Status: Running"
    tailnet=$(tailscale status --json 2>/dev/null | jq -r '.MagicDNSSuffix' 2>/dev/null || echo "unknown")
    echo -e "      Tailnet: $tailnet"
    passed_checks=$((passed_checks + 1))
  else
    echo -e "${RED}${CROSS}${NC}   Status: Not running"
    failed_checks=$((failed_checks + 1))
  fi
fi
check_file "  API Key (sops-nix)" "$HOME/.config/sops-nix/secrets/infrastructure/tailscale_auth_key" false
echo -e "${YELLOW}${WARN}${NC}  Note: CLI fallback works without API key"

echo ""
echo "Kubernetes:"
if check_command "  kubectl" "kubectl" true; then
  if [ -f "$HOME/.kube/config" ]; then
    echo -e "${GREEN}${CHECK}${NC}   Kubeconfig: Present"
    contexts=$(kubectl config get-contexts -o name 2>/dev/null | wc -l)
    echo -e "${GREEN}${CHECK}${NC}   Contexts: $contexts configured"
    kubectl config get-contexts 2>/dev/null | tail -n +2 | while read -r line; do
      echo "      - $line"
    done
    passed_checks=$((passed_checks + 2))
  else
    echo -e "${RED}${CROSS}${NC}   Kubeconfig: Missing"
    failed_checks=$((failed_checks + 1))
  fi
fi

echo ""

# ============================================================================
# 5. Environment Variables Summary
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 5. Environment Variables Summary                                      ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

if env | grep -qiE "(CIVO|DIGITALOCEAN|DREAMHOST|TAILSCALE|ANTHROPIC)_(API_KEY|TOKEN)_FILE"; then
  env | grep -iE "(CIVO|DIGITALOCEAN|DREAMHOST|TAILSCALE|ANTHROPIC)_(API_KEY|TOKEN)_FILE" | sort | while read -r line; do
    echo "  $line"
  done
else
  echo -e "${YELLOW}${WARN}${NC}No *_FILE environment variables found"
  echo "  This is expected if sops-nix secrets aren't loaded yet"
  echo "  Run: ${YELLOW}home-manager switch --flake ~/git/crush-dots#jsullivan2@$(hostname -s)${NC}"
fi

echo ""

# ============================================================================
# 6. Prompt-Pulse Service Status
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 6. Prompt-Pulse Service Status                                        ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

if systemctl --user is-active prompt-pulse &>/dev/null; then
  echo -e "${GREEN}${CHECK}${NC} Daemon: Running"
  echo "  Uptime: $(systemctl --user show -p ActiveEnterTimestamp prompt-pulse --value | xargs -I{} date -d "{}" +'%Y-%m-%d %H:%M:%S')"
  passed_checks=$((passed_checks + 1))
else
  echo -e "${RED}${CROSS}${NC} Daemon: Not running"
  echo "  Start: ${YELLOW}systemctl --user start prompt-pulse${NC}"
  failed_checks=$((failed_checks + 1))
fi

if [ -f "$HOME/.local/state/log/prompt-pulse.log" ]; then
  echo -e "${GREEN}${CHECK}${NC} Log File: Present"
  echo "  Path: $HOME/.local/state/log/prompt-pulse.log"
  echo "  Size: $(du -h "$HOME/.local/state/log/prompt-pulse.log" | cut -f1)"
else
  echo -e "${YELLOW}${WARN}${NC}Log File: Not found (daemon may not have run yet)"
fi

echo ""

# ============================================================================
# Summary
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo -e "Total Checks: $total_checks"
echo -e "${GREEN}Passed:${NC} $passed_checks"
echo -e "${RED}Failed:${NC} $failed_checks"

if [ "$failed_checks" -eq 0 ]; then
  echo ""
  echo -e "${GREEN}${CHECK} All critical checks passed!${NC}"
  echo ""
  echo "Next steps:"
  echo "  1. Start the daemon: ${YELLOW}systemctl --user start prompt-pulse${NC}"
  echo "  2. Check status: ${YELLOW}prompt-pulse status${NC}"
  echo "  3. Open TUI: ${YELLOW}prompt-pulse tui${NC}"
  exit 0
else
  echo ""
  echo -e "${RED}${CROSS} Some checks failed. Review the errors above.${NC}"
  echo ""
  echo "Common fixes:"
  echo "  • Missing secrets: ${YELLOW}home-manager switch --flake ~/git/crush-dots#jsullivan2@$(hostname -s)${NC}"
  echo "  • Missing age key: ${YELLOW}ssh-to-age -private-key < ~/.ssh/id_ed25519 > ~/.config/sops/age/keys.txt${NC}"
  echo "  • AWS not configured: ${YELLOW}sudo dnf install awscli2 && aws configure${NC}"
  echo ""
  echo "For detailed setup: ${YELLOW}cat ~/git/crush-dots/cmd/prompt-pulse/docs/SECRETS_SETUP.md${NC}"
  exit 1
fi
