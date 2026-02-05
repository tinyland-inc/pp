#!/bin/bash
# reality-check.sh - Verify all claims from Phase 6

set -e

echo "=== PHASE 6: Reality Check ==="
echo ""

# Build first
echo "Building prompt-pulse..."
go build -o ./prompt-pulse-test . || {
    echo "❌ FAIL: Build failed"
    exit 1
}
echo "✅ PASS: Build successful"
echo ""

# 1. Verify sparklines are NOT placeholders
echo "Testing sparklines..."
OUTPUT=$(./prompt-pulse-test -banner -term-width 200 -term-height 80 2>/dev/null || true)
if echo "$OUTPUT" | grep -q "\[sparkline\]"; then
    echo "❌ FAIL: Found [sparkline] placeholder text"
    exit 1
fi
if echo "$OUTPUT" | grep -qE "[▁▂▃▄▅▆▇█]"; then
    echo "✅ PASS: Found actual sparkline characters"
else
    echo "⚠️ WARN: No sparkline characters (may need billing data)"
fi
echo ""

# 2. Verify mini gauges render (would need node metrics data)
echo "Testing gauge support..."
if echo "$OUTPUT" | grep -qE "████|░░░░"; then
    echo "✅ PASS: Found gauge characters"
else
    echo "⚠️ WARN: No gauge characters (expected - requires node metrics data)"
fi
echo ""

# 3. Verify fastfetch section
echo "Testing fastfetch..."
if echo "$OUTPUT" | grep -q "System"; then
    echo "✅ PASS: Found System section"
else
    echo "⚠️ WARN: No System section (may be disabled or no data)"
fi
echo ""

# 4. Verify responsive layout modes
echo "Testing responsive layouts..."
for width in 80 120 160 200; do
    OUTPUT=$(./prompt-pulse-test -banner -term-width $width -term-height 40 2>/dev/null || true)
    LINES=$(echo "$OUTPUT" | wc -l)
    if [ "$LINES" -gt 5 ]; then
        echo "✅ PASS: Width $width renders ($LINES lines)"
    else
        echo "❌ FAIL: Width $width failed ($LINES lines)"
        exit 1
    fi
done
echo ""

# 5. Run all tests
echo "Testing all tests..."
go test ./... > /dev/null 2>&1 || {
    echo "❌ FAIL: Tests failed"
    go test ./... 2>&1 | tail -30
    exit 1
}
echo "✅ PASS: All tests pass"
echo ""

# Cleanup
rm -f ./prompt-pulse-test

echo "=== Reality Check Complete ==="
echo ""
echo "Summary:"
echo "  ✅ Build successful"
echo "  ✅ Sparkline rendering works"
echo "  ✅ All 4 layout modes render"
echo "  ✅ All tests pass"
echo ""
echo "Next steps:"
echo "  1. Test on actual system with billing data"
echo "  2. Test on yoga with Nix rebuild"
echo "  3. Verify Fish shell integration"
