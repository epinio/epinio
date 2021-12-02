# shellcheck shell=bash
# shellcheck disable=SC2034

# All generated files should be placed in $OUTPUT_DIR, which is .gitignored.
OUTPUT_DIR="${PWD}/output"
mkdir -p "${OUTPUT_DIR}"

# Temporary files (e.g. downloads) should go to $TEMP_DIR.
TEMP_DIR="${OUTPUT_DIR}/tmp"
mkdir -p "${TEMP_DIR}"

# All downloaded tools will be installed into $TOOLS_DIR.
TOOLS_DIR="${OUTPUT_DIR}/bin"
mkdir -p "${TOOLS_DIR}"

# Python3 virtual env goes into $VENV_DIR
VENV_DIR="${OUTPUT_DIR}/venv"
mkdir -p "${VENV_DIR}"

# UNAME should be DARWIN, LINUX, or WINDOWS.
UNAME="$(uname | tr "[:lower:]" "[:upper:]")"

# Source all tool definitions.
TOOLS=()
# shellcheck disable=SC2044
for TOOL in $(find scripts/tools/*.sh); do
    # shellcheck disable=SC1090
    source "${TOOL}"
done

# Sort tools alphabetically and remove duplicates (there shouldn't be any).
# shellcheck disable=SC2207
TOOLS=($(printf '%s\n' "${TOOLS[@]}" | sort | uniq))

# Have a cache of tool status, so that require_tools will not repeatedly try to
# determine if the same tool has been installed.  This also lets us catch when
# a tool (possibly through others) end up requiring itself.
declare -A TOOL_STATUS

# require_tools makes sure all required tools are available. If the current version
# is too old (or doesn't match the required version exactly when PINNED_TOOLS is set),
# then the tool is downloaded and installed into $TOOLS_DIR.
function require_tools {
    local status
    local tool

    for tool in "$@"; do
        if [[ -z "${tool}" ]]; then
            continue
        fi
        case "${TOOL_STATUS["${tool}"]:-}" in
            "")
                TOOL_STATUS["${tool}"]="installing"
                # Make sure additional prerequisites for the tool are also
                # available.  They must be installed first because they might be
                # needed to install the tool itself.  In this case we *want*
                # word-splitting.
                # shellcheck disable=SC2046
                require_tools $(var_lookup "${tool}_requires")
                if ! status=$(tool_status "${tool}"); then
                    tool_install "${tool}"

                    if ! status="$(tool_status "${tool}")"; then
                        red "Could not install ${tool}"
                        die "${status}"
                    fi
                fi
                TOOL_STATUS["${tool}"]="installed"
                ;;
            installed)
                # Do nothing
                ;;
            installing)
                die "Recursion when installing ${tool}"
                ;;
        esac
    done
}

# tool_status checks the current installation status of a single tool. It return
# a status string, that can be displayed to the user. The return value is either 0
# when an acceptable version of the tool is available, or 1 when the correct
# version needs to be installed.
function tool_status {
    local tool=$1
    local version
    local rc=0
    local status

    version="$(tool_version "${tool}")"
    if [[ "${version}" =~ ^installed|internal|missing$ ]]; then
        if [ "${version}" = "missing" ]; then
            rc=1
        fi
        status="is ${version}"
    else
        status="version is ${version}"
        local minimum
        minimum="$(var_lookup "${tool}_version")"
        if [ -n "${minimum}" ]; then
            case "$(ruby -e "puts Gem::Version.new('${minimum}') <=> Gem::Version.new('${version}')")" in
                -1)
                    status="${status} (newer than ${minimum})"
                    # For PINNED_TOOLS only an exact match is a success (if there is a download URL).
                    if [[ -n "${PINNED_TOOLS:-}" && -n "$(var_lookup "${tool}_url_${UNAME}")" ]]; then
                        rc=1
                    fi
                    ;;
                0)
                    # PINNED_TOOLS *must* be installed in $TOOLS_DIR $VENV_DIR/bin.
                    if [[ -n "${PINNED_TOOLS:-}" && ! -x "${TOOLS_DIR}/$(exe_name "${tool}")" && ! -x "${VENV_DIR}/bin/$(exe_name "${tool}")" ]];
                    then
                        status="${status} (but not installed in ${TOOLS_DIR} or ${VENV_DIR}/bin)"
                        rc=1
                    fi
                    ;;
                1|*)
                    status="${status} (older than ${minimum})"
                    rc=1
                    ;;
            esac
        fi
    fi
    status="${tool} ${status}"

    if [ $rc -eq 0 ]; then
        status="$(green "${status}")"
    else
        status="$(red "${status}")"
    fi
    echo "${status}"
    return ${rc}
}

# tool_version returns the semantic version of the installed tool. I will
# return "internal" for tools implemented as aliases/functions, "missing"
# for tools that cannot be found, and "installed" if the version cannot be
# determined. It is a fatal error if the version cannot be determined for
# a tool that defines a minimum required version.
function tool_version {
    local tool=$1
    local version=""
    local tool_type
    local minimum_version

    tool_type="$(type -t "${tool}")"
    minimum_version="$(var_lookup "${tool}_version")"

    # (Maybe) determine installed version of the tool.
    if [ -z "${tool_type}" ]; then
        echo "missing"
    else
        # Call custom tool version function, if defined.
        if [ -n "$(type -t "${tool}_version")" ]; then
            version="$("${tool}_version")"
        # only call default "$tool version" command if minimum version is defined.
        elif [[ "${tool_type}" = "file" && -n "${minimum_version}" ]]; then
            version="$("${tool}" version)"
        fi

        # Version number must have at least a single dot.
        if [[ "${version}" =~ [0-9]+(\.[0-9]+)+(-[0-9]+)? ]]; then
            echo "${BASH_REMATCH[0]}"
        elif [ "${version}" = "missing" ]; then
            echo "${version}"
        else
            if [ -n "${minimum_version}" ]; then
                die "Cannot determine '${tool}' version (requires ${minimum_version})"
            fi
            case "${tool_type}" in
                file)
                    echo "installed"
                    ;;
                '')
                    echo "missing"
                    ;;
                *)
                    echo "internal"
                    ;;
            esac
        fi
    fi
}

function tool_install {
    local tool=$1
    local sha256
    local url
    local version
    version="$(var_lookup "${tool}_version")"

    blue "Installing ${tool}"

    # Look for custom install command first (e.g. for Python module install via pip).
    if [ -n "$(type -t "${tool}_install")" ]; then
        eval "${tool}_install"
        return
    fi

    require_tools file gzip sha256sum xz

    url="$(var_lookup "${tool}_url_${UNAME}")"
    if [ -z "${url}" ]; then
        die "Can't find URL for ${tool}-${version}"
    fi

    local output="${TEMP_DIR}/output"
    curl -s -L "${url//\{version\}/${version}}" -o "${output}"

    sha256="$(var_lookup "${tool}_sha256_${UNAME}")"
    if [ -n "${sha256}" ] &&  ! echo "${sha256} ${output}" | sha256sum --check --status; then
        die "sha256 for ${url} does not match ${sha256}"
    fi

    local install_location
    install_location="${TOOLS_DIR}/$(exe_name "${tool}")"
    # Keep previous version in case installation fails.
    if [ -f "${install_location}" ]; then
        mv "${install_location}" "${install_location}.prev"
    fi

    if [[ "$(file "${output}")" =~ "gzip compressed" ]]; then
        mv "${output}" "${output}.gz"
        gzip -d "${output}.gz"
    fi

    if [[ "$(file "${output}")" =~ "XZ compressed" ]]; then
        mv "${output}" "${output}.xz"
        xz -d "${output}.xz"
    fi

    local file_type
    file_type="$(file "${output}")"
    case "${file_type}" in
        *executable*)
            mv "${output}" "${install_location}"
            ;;
        *tar*)
            local outdir="${TEMP_DIR}/outdir"
            mkdir -p "${outdir}"
            tar xf "${output}" -C "${outdir}"
            find "${outdir}" -name "$(exe_name "${tool}")" -exec cp {} "${install_location}" \;
            if [ -f "${install_location}" ]; then
                rm -rf "${output}" "${outdir}"
            fi
            ;;
        *)
            die "Unsupported file type of ${output}:\n${file_type}"
            ;;
    esac

    if [ -f "${install_location}" ]; then
        chmod +x "${install_location}"
    else
        if [ -f "${install_location}.prev" ]; then
            mv "${install_location}.prev" "${install_location}"
        fi
        die "Installation of ${tool} failed (previous version may have been restored)"
    fi
}

# exe_name return the filename for the tool executable (including .exe extension on Windows)
function exe_name {
    if [ "${UNAME}" = "WINDOWS" ]; then
        echo "$1.exe"
    else
        echo "$1"
    fi
}
