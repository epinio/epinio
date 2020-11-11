# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(j2y y2j)

function j2y {
    ruby -rjson -ryaml -e "puts JSON.load(STDIN.read).to_yaml.gsub(\"---\n\", '')"
}

function y2j {
    ruby -rjson -ryaml -e "puts YAML.load(STDIN.read).to_json"
}

J2Y_REQUIRES="ruby"
Y2J_REQUIRES="ruby"
