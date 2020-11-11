# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(ruby)

RUBY_VERSION=2.4

function ruby_version { ruby --version; }
