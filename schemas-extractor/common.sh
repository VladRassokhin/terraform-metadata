#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

CUR="$(pwd)"

if [[ ! -f "$config_file" ]]; then
  echo "File '$config_file' required to operate"
  exit 1
fi
if [[ -z "$GOPATH" ]]; then
  echo 'GOPATH env variable required to operate'
  exit 1
fi

function jq_get() {
  name="$1"
  prop="$2"
  jq -r ".\"$name\".$prop // .__NAME__.$prop // \"\" " <"$CUR/$config_file" | sed -e "s/__NAME__/$name/"
}

function trim() {
    local var="$*"
    # remove leading whitespace characters
    var="${var#"${var%%[![:space:]]*}"}"
    # remove trailing whitespace characters
    var="${var%"${var##*[![:space:]]}"}"
    printf '%s' "$var"
}

function update_or_clone() {
  repository="$1"
  location="$GOPATH/src/$repository"
  if [[ -d "$location" ]]; then
    echo "Updating $repository"
    git -C "$location" fetch --tags >/dev/null 2>&1 || echo "ERROR: Failed to update '$repository'"
  else
    echo "Cloning $repository"
    git clone --quiet "https://$repository" "$location" >/dev/null 2>&1
  fi
}

function update_all() {
  if [[ -z "${UPDATE_SKIP:-}" ]]; then
    repositories=()
    while IFS= read -r p; do
      if [[ "$p" == "__NAME__" ]]; then
        continue
      fi

      # If a list is provided via make, then only update specific repos
      if [ ! -z ${ONLY+x} ]; then 
        if [[ " ${CHECK_LIST[*]} " != *"${p}"* ]]; then
          continue
        fi
      fi

      skip_generation="$(jq_get "$p" 'skip_generation')"
      if [[ $skip_generation == "true" ]]; then
        continue
      fi
      repository="$(jq_get "$p" 'repository')"
      repositories+=("$repository")
    done < <(jq -r 'keys[]' <"$CUR/$config_file")
    if [ ! ${#repositories[@]} -eq 0 ]; then
      mapfile -t uniq < <(sort -u <<<"${repositories[*]}")
      for repo in "${uniq[@]}"; do
        repo=$(trim "$repo")
        if [[ "${UPDATE_PARALLEL:-}" == "1" ]]; then
          (
            update_or_clone "$repo"
          ) &
        else
          update_or_clone "$repo"
        fi
      done
    else
      echo "No matching repos found. Provided list: ${CHECK_LIST[*]}"
    fi
  fi
}

function generate_one() {
  mkdir -p "$logs"
  log_file="$logs/$1.log"
  set +e
  eval ${2:-} go run ${3:-} generate-schema/generate-schema.go 2>&1 | tee "$log_file"
  ec=$?
  if [[ $ec -eq 0 ]]; then
    rm "$log_file"
  else
    echo "$1" >>"$failures"
  fi
  echo "Finished $1"
  set -e
}
