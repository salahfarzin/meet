#!/bin/bash

set -e

echo "🔍 Running Quality Gate Checks..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    local status=$1
    local message=$2
    if [ "$status" -eq 0 ]; then
        echo -e "${GREEN}✅ $message${NC}"
    else
        echo -e "${RED}❌ $message${NC}"
        return 1
    fi
}

# Check if required tools are installed
check_tools() {
    echo "Checking required tools..."

    if ! command -v golangci-lint &> /dev/null; then
        echo -e "${RED}❌ golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest${NC}"
        exit 1
    fi

    if ! command -v gosec &> /dev/null; then
        echo -e "${RED}❌ gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest${NC}"
        exit 1
    fi

    if ! command -v gocyclo &> /dev/null; then
        echo -e "${RED}❌ gocyclo not found. Install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest${NC}"
        exit 1
    fi

    print_status 0 "All required tools are installed"
}

# Run linting
run_lint() {
    echo "Running golangci-lint..."
    if golangci-lint run --timeout=5m --skip-dirs proto; then
        print_status 0 "Linting passed"
    else
        print_status 1 "Linting failed"
        return 1
    fi
}

# Run tests with coverage
run_tests() {
    echo "Running tests with coverage..."
    # Get list of packages to test, excluding proto and cmd
    PACKAGES=$(go list ./... | grep -v proto | grep -v cmd | tr '\n' ' ')
    if go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic $PACKAGES; then
        # Calculate coverage only for the packages we tested (excluding main package)
        TOTAL_COVERAGE=0
        PACKAGE_COUNT=0

        while IFS= read -r line; do
            if [[ $line == github.com/salahfarzin/meet/* ]] && [[ $line != github.com/salahfarzin/meet$ ]]; then
                COVERAGE_PERCENT=$(echo "$line" | awk '{print $3}' | sed 's/%//')
                TOTAL_COVERAGE=$(echo "$TOTAL_COVERAGE + $COVERAGE_PERCENT" | bc -l)
                PACKAGE_COUNT=$((PACKAGE_COUNT + 1))
            fi
        done < <(go tool cover -func=coverage/coverage.out | grep -E "^github\.com/salahfarzin/meet/")

        if [ $PACKAGE_COUNT -gt 0 ]; then
            COVERAGE=$(echo "scale=1; $TOTAL_COVERAGE / $PACKAGE_COUNT" | bc -l)
        else
            COVERAGE=0
        fi

        echo "Test coverage: $COVERAGE%"

        if (( $(echo "$COVERAGE < 85.0" | bc -l) )); then
            echo -e "${RED}❌ Test coverage is below 85%: $COVERAGE%${NC}"
            return 1
        fi

        print_status 0 "Tests passed with sufficient coverage: $COVERAGE%"
    else
        print_status 1 "Tests failed"
        return 1
    fi
}

# Run security scan
run_security() {
    echo "Running security scan..."
    if gosec -no-fail -exclude-dir=proto ./...; then
        print_status 0 "Security scan passed"
    else
        print_status 1 "Security scan found issues"
        return 1
    fi
}

# Check code complexity
check_complexity() {
    echo "Checking code complexity..."
    COMPLEX_FUNCTIONS=$(gocyclo -over 10 $(find . -name "*.go" -not -path "./proto/*") | wc -l)

    if [ "$COMPLEX_FUNCTIONS" -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Found functions with high complexity (cyclomatic complexity > 10):${NC}"
        gocyclo -over 10 $(find . -name "*.go" -not -path "./proto/*")
        echo -e "${YELLOW}Consider refactoring these functions to improve maintainability.${NC}"
    else
        print_status 0 "Code complexity is within acceptable limits"
    fi
}

# Check for TODO/FIXME comments
check_todos() {
    echo "Checking for TODO/FIXME comments..."
    TODO_COUNT=$(grep -r "TODO\|FIXME\|XXX" --include="*.go" --exclude-dir=vendor --exclude-dir=proto . | wc -l)

    if [ "$TODO_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Found $TODO_COUNT TODO/FIXME comments:${NC}"
        grep -r "TODO\|FIXME\|XXX" --include="*.go" --exclude-dir=vendor --exclude-dir=proto . | head -10
        if [ "$TODO_COUNT" -gt 10 ]; then
            echo -e "${YELLOW}... and $(($TODO_COUNT - 10)) more${NC}"
        fi
        echo -e "${YELLOW}Consider addressing these technical debt items.${NC}"
    else
        print_status 0 "No TODO/FIXME comments found"
    fi
}

# Main execution
main() {
    echo "🚀 Starting Quality Gate..."
    echo

    check_tools
    echo

    local failed=0

    run_lint || failed=1
    echo

    run_tests || failed=1
    echo

    run_security || failed=1
    echo

    check_complexity
    echo

    check_todos
    echo

    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}🎉 All quality gates passed! Code is ready for commit/merge.${NC}"
        exit 0
    else
        echo -e "${RED}💥 Quality gate failed! Please fix the issues above before committing.${NC}"
        exit 1
    fi
}

main "$@"