#!/bin/bash
# Script to verify all BDD tests are discoverable and runnable
# This can be used in CI to validate BDD test coverage

set -e

echo "=== BDD Test Verification Script ==="
echo "Verifying all BDD tests are present and runnable..."

# Check core framework BDD tests
echo ""
echo "--- Core Framework BDD Tests ---"
if go test -list "TestApplicationLifecycle|TestConfigurationManagement" . 2>/dev/null | grep -q "Test"; then
    echo "✅ Core BDD tests found and accessible"
    go test -list "TestApplicationLifecycle|TestConfigurationManagement" . 2>/dev/null | grep "Test"
else
    echo "❌ Core BDD tests not found or not accessible"
    exit 1
fi

# Check module BDD tests
echo ""
echo "--- Module BDD Tests ---"
total_modules=0
bdd_modules=0

for module in modules/*/; do
    if [ -f "$module/go.mod" ]; then
        module_name=$(basename "$module")
        total_modules=$((total_modules + 1))
        
        cd "$module"
        if go test -list ".*BDD|.*Module" . 2>/dev/null | grep -q "Test"; then
            echo "✅ $module_name: BDD tests found"
            go test -list ".*BDD|.*Module" . 2>/dev/null | grep "Test" | head -3
            bdd_modules=$((bdd_modules + 1))
        else
            echo "⚠️  $module_name: No BDD tests found"
        fi
        cd - >/dev/null
    fi
done

echo ""
echo "=== Summary ==="
echo "Total modules checked: $total_modules"
echo "Modules with BDD tests: $bdd_modules"

if [ $bdd_modules -gt 0 ]; then
    echo "✅ BDD test verification completed successfully"
    echo "Coverage: $(( bdd_modules * 100 / total_modules ))% of modules have BDD tests"
else
    echo "❌ No BDD tests found in any modules"
    exit 1
fi