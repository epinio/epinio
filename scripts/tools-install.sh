#!/usr/bin/env bash

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
