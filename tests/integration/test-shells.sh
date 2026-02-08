#!/bin/bash
# Cross-shell integration tests for prompt-pulse
# Validates that shell integration scripts are generated correctly
# for all supported shells (bash, zsh, fish).

set -euo pipefail

# Determine script directory and prompt-pulse binary location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Try to find prompt-pulse binary
if [[ -x "${PROJECT_ROOT}/prompt-pulse" ]]; then
    PROMPT_PULSE="${PROJECT_ROOT}/prompt-pulse"
elif command -v prompt-pulse &>/dev/null; then
    PROMPT_PULSE="$(command -v prompt-pulse)"
else
    echo "ERROR: prompt-pulse binary not found"
    echo "Build it with: cd ${PROJECT_ROOT} && go build"
    exit 1
fi

echo "Using prompt-pulse: ${PROMPT_PULSE}"

# Test output directory
TEST_OUTPUT_DIR="${TMPDIR:-/tmp}/prompt-pulse-shell-tests"
mkdir -p "${TEST_OUTPUT_DIR}"

# Track test results
PASSED=0
FAILED=0

# Test helper functions
pass() {
    echo "  PASS: $1"
    PASSED=$((PASSED + 1))
}

fail() {
    echo "  FAIL: $1"
    FAILED=$((FAILED + 1))
}

# Shells to test
SHELLS=("bash" "zsh" "fish")

echo ""
echo "=== Cross-Shell Integration Tests ==="
echo ""

for shell in "${SHELLS[@]}"; do
    echo "Testing ${shell}..."

    OUTPUT_FILE="${TEST_OUTPUT_DIR}/shell-${shell}.txt"

    # Test 1: Shell template generation
    if "${PROMPT_PULSE}" --shell "${shell}" > "${OUTPUT_FILE}" 2>&1; then
        pass "${shell} template generated"
    else
        fail "${shell} template generation failed"
        cat "${OUTPUT_FILE}"
        continue
    fi

    # Test 2: pp-banner function exists
    if grep -q "pp-banner" "${OUTPUT_FILE}"; then
        pass "${shell} has pp-banner function"
    else
        fail "${shell} missing pp-banner function"
    fi

    # Test 3: Session ID support (PPULSE_SESSION_ID)
    if grep -q "PPULSE_SESSION_ID" "${OUTPUT_FILE}"; then
        pass "${shell} has session ID support"
    else
        fail "${shell} missing session ID support"
    fi

    # Test 4: Core functions exist
    for func in "pp-status" "pp-tui" "pp-daemon-start" "pp-daemon-stop"; do
        if grep -q "${func}" "${OUTPUT_FILE}"; then
            pass "${shell} has ${func} function"
        else
            fail "${shell} missing ${func} function"
        fi
    done

    # Test 5: Shell-specific keybinding
    case "${shell}" in
        bash)
            if grep -q "bind -x" "${OUTPUT_FILE}"; then
                pass "${shell} has correct keybinding (bind -x)"
            else
                fail "${shell} missing keybinding"
            fi
            ;;
        zsh)
            if grep -q "bindkey" "${OUTPUT_FILE}" && grep -q "zle -N" "${OUTPUT_FILE}"; then
                pass "${shell} has correct keybinding (bindkey/zle)"
            else
                fail "${shell} missing keybinding"
            fi
            ;;
        fish)
            if grep -q "bind" "${OUTPUT_FILE}"; then
                pass "${shell} has correct keybinding (bind)"
            else
                fail "${shell} missing keybinding"
            fi
            ;;
    esac

    # Test 6: prompt-pulse binary reference
    if grep -q "prompt-pulse" "${OUTPUT_FILE}"; then
        pass "${shell} references prompt-pulse binary"
    else
        fail "${shell} missing prompt-pulse binary reference"
    fi

    echo ""
done

# Summary
echo "=== Test Summary ==="
echo "Passed: ${PASSED}"
echo "Failed: ${FAILED}"
echo ""

# Cleanup
rm -rf "${TEST_OUTPUT_DIR}"

if [[ ${FAILED} -gt 0 ]]; then
    echo "Some tests failed!"
    exit 1
fi

echo "All tests passed!"
exit 0
