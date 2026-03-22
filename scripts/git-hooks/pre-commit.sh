#!/bin/bash

# niceboy Pre-Commit Hook
# 1. Blocks commits if it detects potential API keys or secrets in staged YAML/Go files.
# 2. Ensures the Go test suite passes.

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}🔍 Running niceboy Pre-Commit Security & QA Checks...${NC}"

# 1. Secret Scan
# We look for "key:" or "secret:" lines that don't contain placeholders like "YOUR_" or "RET_HERE"
# We only scan staged files (.yaml, .go)
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(yaml|yml|go)$')

if [ -n "$STAGED_FILES" ]; then
    for file in $STAGED_FILES; do
        # Exclude example and documentation files
        if [[ "$file" == *"example"* ]] || [[ "$file" == *"docs/"* ]]; then
            continue
        fi

        # Grep for potential secrets:
        # - key: [some value that isn't YOUR_]
        # - secret: [some value that isn't YOUR_]
        # - Look for Long alphanumeric strings (e.g. 32+ characters) that aren't placeholders
        SECRET_MATCHES=$(git diff --cached "$file" | grep -E '^\+.*(key|secret|API_KEY|API_SECRET)' | grep -vE '(YOUR_|RET_HERE)')
        
        if [ -n "$SECRET_MATCHES" ]; then
            echo -e "${RED}❌ SECURITY ALERT: Potential API Secret found in staged file: $file${NC}"
            echo -e "${RED}Matches:${NC}\n$SECRET_MATCHES"
            echo -e "${YELLOW}If this is a false positive, use 'git commit --no-verify' or update the placeholder detector.${NC}"
            exit 1
        fi
    done
    echo -e "${GREEN}✅ No leaked secrets detected in staged files.${NC}"
else
    echo -e "${YELLOW}No relevant staged files to scan for secrets.${NC}"
fi

# 2. Run Tests
echo -e "${YELLOW}🧪 Running Go Test Suite...${NC}"
go test ./...
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ FAILED: Unit tests must pass before committing.${NC}"
    exit 1
fi
echo -e "${GREEN}✅ All tests passed.${NC}"

echo -e "${GREEN}🚀 Commit Approved!${NC}"
exit 0
