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

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}SUCCESS: $1${NC}" >&2
}

print_warning() {
    echo -e "${YELLOW}WARNING: $1${NC}" >&2
}

print_info() {
    echo "INFO: $1" >&2
}

# Get the Epinio version from version.go file
get_epinio_version_from_code() {
    local version_file="internal/version/version.go"
    
    if [ ! -f "$version_file" ]; then
        echo "not-found"
        return 1
    fi
    
    # Extract ChartVersion from version.go
    # Look for: var ChartVersion = "v1.2.3"
    local version=$(grep 'var ChartVersion =' "$version_file" | sed 's/.*= *"\(.*\)".*/\1/' | tr -d ' ')
    
    if [ -z "$version" ] || [ "$version" = "v0.0.0-dev" ]; then
        # If ChartVersion is not set or is dev, fall back to git tag
        return 1
    fi
    
    echo "$version"
}

# Get the Epinio version from git tags or environment
get_epinio_version() {
    # Priority order:
    # 1. Explicit EPINIO_VERSION env var (for testing and overrides)
    # 2. Version from version.go (for PR validation)
    # 3. GITHUB_REF_NAME (for release tags in GitHub Actions)
    # 4. Latest git tag (fallback)
    
    if [ -n "$EPINIO_VERSION" ]; then
        echo "$EPINIO_VERSION"
    else
        # Try to get from version.go first (for PR validation)
        local code_version=$(get_epinio_version_from_code 2>/dev/null)
        if [ $? -eq 0 ] && [ -n "$code_version" ]; then
            echo "$code_version"
        elif [ -n "$GITHUB_REF_NAME" ]; then
            echo "$GITHUB_REF_NAME"
        else
            git describe --tags --abbrev=0 2>/dev/null || echo "unknown"
        fi
    fi
}

# Get the Chart version from helm-charts submodule
get_chart_version() {
    local chart_file="$1"
    
    if [ ! -f "$chart_file" ]; then
        echo "not-found"
        return 1
    fi
    
    # Extract version from Chart.yaml (handle both 'version:' and 'appVersion:')
    local chart_version=$(grep '^version:' "$chart_file" | awk '{print $2}' | tr -d '"' | tr -d "'")
    local app_version=$(grep '^appVersion:' "$chart_file" | awk '{print $2}' | tr -d '"' | tr -d "'")
    
    # We care about the appVersion as it should match the Epinio release version
    if [ -n "$app_version" ]; then
        echo "$app_version"
    else
        echo "$chart_version"
    fi
}

# Fetch the Chart version from remote helm-charts repository
get_chart_version_from_remote() {
    local version="$1"
    local helm_charts_repo="https://raw.githubusercontent.com/epinio/helm-charts/main/chart/epinio/Chart.yaml"
    
    print_info "Fetching Chart.yaml from remote repository..."
    
    if command -v curl &> /dev/null; then
        local chart_content=$(curl -sL "$helm_charts_repo" 2>/dev/null)
    elif command -v wget &> /dev/null; then
        local chart_content=$(wget -qO- "$helm_charts_repo" 2>/dev/null)
    else
        print_error "Neither curl nor wget is available"
        return 1
    fi
    
    if [ -z "$chart_content" ]; then
        print_error "Failed to fetch Chart.yaml from remote"
        return 1
    fi
    
    local app_version=$(echo "$chart_content" | grep '^appVersion:' | awk '{print $2}' | tr -d '"' | tr -d "'")
    
    if [ -n "$app_version" ]; then
        echo "$app_version"
    else
        echo "unknown"
    fi
}

# Normalize version (remove 'v' prefix if present for comparison)
normalize_version() {
    echo "$1" | sed 's/^v//'
}

# Main validation logic
main() {
    local mode="${1:-check}"  # check, warn, or strict
    local chart_file="${2:-helm-charts/chart/epinio/Chart.yaml}"
    
    print_info "Validating version parity between Epinio release and Helm chart..."
    print_info "Mode: $mode"
    echo ""
    
    # Get Epinio version
    local epinio_version=$(get_epinio_version)
    
    # Determine the source of the version
    if [ -n "$EPINIO_VERSION" ]; then
        print_info "Epinio version: $epinio_version (from EPINIO_VERSION env var)"
    else
        local code_version=$(get_epinio_version_from_code 2>/dev/null)
        if [ $? -eq 0 ] && [ -n "$code_version" ] && [ "$code_version" != "v0.0.0-dev" ]; then
            print_info "Epinio version: $epinio_version (from internal/version/version.go)"
        elif [ -n "$GITHUB_REF_NAME" ]; then
            print_info "Epinio version: $epinio_version (from GITHUB_REF_NAME)"
        else
            print_info "Epinio version: $epinio_version (from git tag)"
        fi
    fi
    
    # Initialize helm-charts submodule if not already initialized
    if [ -d "helm-charts" ] && [ ! -f "$chart_file" ]; then
        print_info "Helm-charts submodule not initialized. Attempting to initialize..."
        git submodule update --init --recursive helm-charts 2>/dev/null || {
            print_warning "Failed to initialize helm-charts submodule"
            print_info "Attempting to fetch Chart version from remote repository..."
            chart_version=$(get_chart_version_from_remote "$epinio_version")
        }
    fi
    
    # Get Chart version
    local chart_version
    if [ -f "$chart_file" ]; then
        chart_version=$(get_chart_version "$chart_file")
        print_info "Chart version (from local): $chart_version"
    else
        chart_version=$(get_chart_version_from_remote "$epinio_version")
        print_info "Chart version (from remote): $chart_version"
    fi
    
    echo ""
    
    # Normalize versions for comparison
    local epinio_version_normalized=$(normalize_version "$epinio_version")
    local chart_version_normalized=$(normalize_version "$chart_version")
    
    # Skip validation if versions are unknown or not found
    if [ "$epinio_version" = "unknown" ] || [ "$chart_version" = "not-found" ] || [ "$chart_version" = "unknown" ]; then
        print_warning "Unable to determine versions for comparison"
        if [ "$mode" = "strict" ]; then
            print_error "Strict mode enabled: version validation failed"
            exit 1
        fi
        print_info "Skipping validation"
        exit 0
    fi
    
    # Compare versions
    if [ "$epinio_version_normalized" = "$chart_version_normalized" ]; then
        print_success "Version parity validated successfully!"
        print_info "  Epinio version: $epinio_version"
        print_info "  Chart version:  $chart_version"
        exit 0
    else
        local message="Version mismatch detected!\n"
        message+="  Epinio version: $epinio_version\n"
        message+="  Chart version:  $chart_version\n"
        message+="\nThis may indicate that:\n"
        message+="  1. The helm chart needs to be updated to match this release\n"
        message+="  2. The wrong version tag is being released\n"
        message+="  3. The helm-charts submodule is out of date\n"
        message+="\nPlease ensure version parity before proceeding with the release."
        
        case "$mode" in
            strict)
                print_error "$message"
                echo ""
                print_error "Strict mode enabled: Release blocked due to version mismatch"
                exit 1
                ;;
            warn)
                print_warning "$message"
                echo ""
                print_warning "Warning mode: Proceeding despite version mismatch"
                exit 0
                ;;
            *)
                print_warning "$message"
                exit 0
                ;;
        esac
    fi
}

# Show usage
usage() {
    cat << EOF
Usage: $0 [MODE] [CHART_FILE]

Validate version parity between Epinio release and Helm chart.

Modes:
  check   - Check and report version parity (default)
  warn    - Show warnings but don't fail
  strict  - Fail if versions don't match (use for release gates)

Arguments:
  CHART_FILE  - Path to Chart.yaml (default: helm-charts/chart/epinio/Chart.yaml)

Environment Variables:
  EPINIO_VERSION  - Override Epinio version (defaults to git describe)
  GITHUB_REF_NAME - Used in GitHub Actions to get the tag name

Examples:
  $0 check
  $0 strict
  $0 warn helm-charts/chart/epinio/Chart.yaml
  EPINIO_VERSION=v1.11.1 $0 strict

EOF
}

# Parse arguments
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
    exit 0
fi

main "$@"
