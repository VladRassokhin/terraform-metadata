#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

function jq_get() {
  name="$1"
  prop="$2"
  config="$3"
  echo $config | jq -r ".\"$name\".$prop // .__NAME__.$prop // \"\" " | sed -e "s/__NAME__/$name/"
}

function trim() {
    local var="$*"
    # remove leading whitespace characters
    var="${var#"${var%%[![:space:]]*}"}"
    # remove trailing whitespace characters
    var="${var%"${var##*[![:space:]]}"}"
    printf '%s' "$var"
}

root_path=$(pwd)
base_location="$root_path/../../schema-extractor-repos"
target_repo=""
logs_path="$root_path/logs"
out_path="$root_path/schemas"

while getopts ":b:t:l:o:" opt; do
  case "${opt}" in
    b )
      base_location="${OPTARG}"
      ;;
    t )
      target_repo="${OPTARG}"
      ;;
    l )
      logs_path="${OPTARG}"
      ;;
    o )
      out_path="${OPTARG}"
      ;;
    \? )
      echo "oops"
      ;;
    : )
      echo "Invalid Option: -$OPTARG requires an argument" 1>&2
      exit 1
      ;;
  esac
done


#
#function update_or_clone() {
#  repository="$1"
#  location="$CUR/../../temp/$repository"
#  if [[ -d "$location" ]]; then
#    echo "Updating $repository"
#    git -C "$location" fetch --tags >/dev/null 2>&1 || echo "ERROR: Failed to update '$repository'"
#  else
#    echo "Cloning $repository"
#    git clone --quiet "https://$repository" "$location" >/dev/null 2>&1
#  fi
#}
#
#function update_all() {
#  if [[ -z "${UPDATE_SKIP:-}" ]]; then
#    repositories=()
#    while IFS= read -r p; do
#      if [[ "$p" == "__NAME__" ]]; then
#        continue
#      fi
#      skip_generation="$(jq_get "$p" 'skip_generation')"
#      if [[ $skip_generation == "true" ]]; then
#        continue
#      fi
#      repository="$(jq_get "$p" 'repository')"
#      repositories+=("$repository")
#    done < <(jq -r 'keys[]' <"$CUR/$config_file")
#    mapfile -t uniq < <(sort -u <<<"${repositories[*]}")
#    for repo in "${uniq[@]}"; do
#      repo=$(trim "$repo")
#      if [[ "${UPDATE_PARALLEL:-}" == "1" ]]; then
#        (
#          update_or_clone "$repo"
#        ) &
#      else
#        update_or_clone "$repo"
#      fi
#    done
#  fi
#}
#
#function generate_one() {
#  mkdir -p "$logs"
#  log_file="$logs/$1.log"
#  set +e
#  eval ${2:-} go run ${3:-} generate-schema.go 2>&1 | tee "$log_file"
#  ec=$?
#  if [[ $ec -eq 0 ]]; then
#    rm "$log_file"
#  else
#    echo "$1" >>"$failures"
#  fi
#  echo "Finished $1"
#  set -e
#}
