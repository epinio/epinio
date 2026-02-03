#!/bin/bash
# Copyright © 2021 - 2023 SUSE LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Test script for validate-version-parity.sh

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
VALIDATION_SCRIPT="$SCRIPT_DIR/validate-version-parity.sh"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_test() {
    echo -e "${YELLOW}TEST: $1${NC}"
}

print_pass() {
    echo -e "${GREEN}✓ PASS: $1${NC}"
}

print_fail() {
    echo -e "${RED}✗ FAIL: $1${NC}"
}

# Make validation script executable
chmod +x "$VALIDATION_SCRIPT"

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Create temporary Chart.yaml for testing
TEMP_DIR=$(mktemp -d)
TEMP_CHART="$TEMP_DIR/Chart.yaml"

cleanup() {
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

# Test 1: Matching versions
print_test "Test 1: Matching versions (v1.0.0 == v1.0.0)"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.0.0
appVersion: v1.0.0
EOF

if EPINIO_VERSION=v1.0.0 "$VALIDATION_SCRIPT" check "$TEMP_CHART" &>/dev/null; then
    print_pass "Matching versions detected correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Failed to detect matching versions"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Matching versions without 'v' prefix
print_test "Test 2: Matching versions without v prefix (1.0.0 == v1.0.0)"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.0.0
appVersion: 1.0.0
EOF

if EPINIO_VERSION=v1.0.0 "$VALIDATION_SCRIPT" check "$TEMP_CHART" &>/dev/null; then
    print_pass "Version normalization works correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Version normalization failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Mismatching versions in check mode (should pass)
print_test "Test 3: Mismatching versions in check mode (should warn but pass)"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.0.0
appVersion: v1.0.0
EOF

if EPINIO_VERSION=v1.1.0 "$VALIDATION_SCRIPT" check "$TEMP_CHART" &>/dev/null; then
    print_pass "Check mode allows mismatch"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Check mode should not fail on mismatch"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Mismatching versions in strict mode (should fail)
print_test "Test 4: Mismatching versions in strict mode (should fail)"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.0.0
appVersion: v1.0.0
EOF

if EPINIO_VERSION=v1.1.0 "$VALIDATION_SCRIPT" strict "$TEMP_CHART" &>/dev/null; then
    print_fail "Strict mode should fail on mismatch"
    TESTS_FAILED=$((TESTS_FAILED + 1))
else
    print_pass "Strict mode correctly fails on mismatch"
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi
echo ""

# Test 5: Mismatching versions in warn mode (should pass)
print_test "Test 5: Mismatching versions in warn mode (should warn but pass)"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.0.0
appVersion: v1.0.0
EOF

if EPINIO_VERSION=v1.1.0 "$VALIDATION_SCRIPT" warn "$TEMP_CHART" &>/dev/null; then
    print_pass "Warn mode allows mismatch"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Warn mode should not fail on mismatch"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 6: Help flag
print_test "Test 6: Help flag"
TESTS_RUN=$((TESTS_RUN + 1))
if "$VALIDATION_SCRIPT" --help &>/dev/null; then
    print_pass "Help flag works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Help flag failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 7: RC versions
print_test "Test 7: Release candidate versions"
TESTS_RUN=$((TESTS_RUN + 1))
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.13.8-rc1
appVersion: v1.13.8-rc1
EOF

if EPINIO_VERSION=v1.13.8-rc1 "$VALIDATION_SCRIPT" check "$TEMP_CHART" &>/dev/null; then
    print_pass "RC versions work correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "RC version validation failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 8: Reading from version.go (mock)
print_test "Test 8: Reading version from version.go"
TESTS_RUN=$((TESTS_RUN + 1))

# Create a mock version.go file
TEMP_VERSION_DIR="$TEMP_DIR/internal/version"
mkdir -p "$TEMP_VERSION_DIR"
cat > "$TEMP_VERSION_DIR/version.go" << 'EOF'
package version

var Version = "v0.0.0-dev"
var ChartVersion = "v1.2.0"
EOF

# Create matching chart
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.2.0
appVersion: v1.2.0
EOF

# Run validation from temp directory to pick up mock version.go
if (cd "$TEMP_DIR" && "$SCRIPT_DIR/validate-version-parity.sh" check "$(basename $TEMP_CHART)" &>/dev/null); then
    print_pass "Successfully reads version from version.go"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_fail "Failed to read version from version.go"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 9: Version.go mismatch detection
print_test "Test 9: Detect mismatch when reading from version.go"
TESTS_RUN=$((TESTS_RUN + 1))

# version.go already has v1.2.0 from test 8
# Create mismatched chart
cat > "$TEMP_CHART" << EOF
apiVersion: v2
name: epinio
version: 1.3.0
appVersion: v1.3.0
EOF

# Run validation from temp directory - should fail in strict mode
if (cd "$TEMP_DIR" && "$SCRIPT_DIR/validate-version-parity.sh" strict "$(basename $TEMP_CHART)" &>/dev/null); then
    print_fail "Should detect mismatch from version.go"
    TESTS_FAILED=$((TESTS_FAILED + 1))
else
    print_pass "Correctly detects mismatch from version.go"
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi
echo ""

# Print summary
echo "================================================"
echo "Test Summary"
echo "================================================"
echo "Tests run:    $TESTS_RUN"
echo -e "${GREEN}Tests passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Tests failed: $TESTS_FAILED${NC}"
else
    echo -e "${GREEN}Tests failed: $TESTS_FAILED${NC}"
fi
echo "================================================"

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
