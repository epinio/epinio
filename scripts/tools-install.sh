#!/usr/bin/env bash
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

function colorize {
    local color=$1
    local text=$2

    # COLOR defaults to true if stdout is a tty.
    if [[ -z "${COLOR:-}" && -t 1 ]]; then
        COLOR=true
    fi

    # XXX Does not really work correctly for `COLOR= make ...`
    if [ -z "${COLOR:-}" ]; then
        printf "%b\n" "${text}"
    else
        printf "\033[${color}m%b\033[0m\n" "${text}"
    fi
}

function green() {
    colorize 32 "$1"
}

function red() {
    colorize 31 "$1"
}

function blue() {
    colorize 34 "$1"
}

function cleanup {
  rm -rf "$TMP_DIR"
}

UNAME="$(uname | tr "[:lower:]" "[:upper:]")"
OUTPUT_DIR="${PWD}/output/bin"
mkdir -p "$OUTPUT_DIR"

TMP_DIR=`mktemp -d`
if [[ ! "$TMP_DIR" || ! -d "$TMP_DIR" ]]; then
  echo "Could not create temp dir"
  exit 1
fi
trap cleanup EXIT


for TOOL in $(find scripts/tools/*.sh); do
    blue "Running ${TOOL}"
    source "${TOOL}"
done

rm -rf "$TMP_DIR"/*
