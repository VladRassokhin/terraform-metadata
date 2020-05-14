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

function update_or_clone() {
  name="$1"
  if [[ "$name" == "__NAME__" ]]; then
    return 0
  fi
  skip_generation="$(jq_get "$name" 'skip_generation')"
  if [[ $skip_generation == "true" ]]; then
    return 0
  fi
  repository="$(jq_get "$name" 'repository')"
  location="$GOPATH/src/$repository"
  if [[ -d "$location" ]]; then
    echo "Updating $name"
    git -C "$location" fetch --tags >/dev/null 2>&1 || echo "ERROR: Failed to update '$name'"
  else
    echo "Cloning $name"
    git clone --quiet "https://$repository" "$location" >/dev/null 2>&1
  fi
}

function update_all() {
  if [[ -z "${UPDATE_SKIP:-}" ]]; then
    while IFS= read -r p; do
      if [[ "${UPDATE_PARALLEL:-}" == "1" ]]; then
        (
          update_or_clone "$p"
        ) &
      else
        update_or_clone "$p"
      fi
    done < <(jq -r 'keys[]' <"$CUR/$config_file")
  fi
}

function generate_one() {
  mkdir -p "$logs"
  log_file="$logs/$1.log"
  set +e
  GO111MODULE=off go run generate-schema/generate-schema.go 2>&1 | tee "$log_file"
  ec=$?
  if [[ $ec -eq 0 ]]; then
    rm "$log_file"
  else
    echo "$1" >>"$failures"
  fi
  echo "Finished $1"
  set -e
}
