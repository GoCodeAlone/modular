#!/bin/bash
for module in modules/*/; do
    if [ -f "$module/go.mod" ]; then
        echo "Fixing $module"
        cd "$module"
        # Fix the dependency line to include a proper version
        sed -i 's/github\.com\/GoCodeAlone\/modular$/github.com\/GoCodeAlone\/modular v0.0.0-00010101000000-000000000000/' go.mod
        # Add replace directive if it doesn't exist
        if ! grep -q "replace github.com/GoCodeAlone/modular" go.mod; then
            echo "" >> go.mod
            echo "replace github.com/GoCodeAlone/modular => ../../" >> go.mod
        fi
        cd - > /dev/null
    fi
done
