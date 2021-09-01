#!/usr/bin/env bash

config_file='functions.json'

. common.sh

logs="$CUR/logs"
failures="$CUR/functions-failure.txt"
out="$CUR/schemas"

mkdir -p "$out"
rm -f "$failures"

update_or_clone github.com/hashicorp/terraform
echo "Repo updated"
echo

function process_functions() {
  name="$1"
  if [[ "$name" == "__NAME__" ]]; then
    return 0
  fi
  skip_generation="$(jq_get "$name" 'skip_generation')"
  if [[ $skip_generation == "true" ]]; then
    return 0
  fi
  repository="$(jq_get "$name" 'repository')"
  use_master="$(jq_get "$name" 'use_master')"
  location="$GOPATH/src/$repository"

  if [[ ! -d "$location" ]]; then
    echo "$location doesn't exist, skipping"
    return 1
  fi

  if output=$(git -C "$location" status --untracked-files=no --porcelain) && [[ -n "$output" ]]; then
    if [[ -z "${RESET_REPOS:-}" ]]; then
      mkdir -p "$logs"
      echo "git working copy '$location' is not clear, cannot proceed" | tee "$logs/$name.log"
      echo "$name" >>"$failures"
      return 2
    else
      git -C "$location" reset --hard
      git -C "$location" clean -fdx
    fi
  fi

  pushd "$location" >/dev/null || return

  echo "Preparing $name"

  if [[ "$use_master" == "true" ]]; then
    echo "Using master"
    latest="master"
  else
    # All tags:
    echo "Repository newest tags:"
    git tag -l --sort=-v:refname | grep -v alpha | head -n 5
    latest=$(git tag -l --sort=-v:refname | grep -v alpha  | head -n 1)
    if [[ -z "$latest" ]]; then
      echo "There's no tags in $name, will use current state"
    else
      echo "Will generate schema for tag '$latest'"
    fi
  fi

  [[ -n "$latest" ]] && git checkout -q "$latest"

  if ! git rev-parse HEAD &>/dev/null; then
    echo "'git rev-parse HEAD' failed, skipping $name"
    return
  fi

  revision="$(git describe --tags)"
  if [[ -n "$latest" ]] && [[ "$revision" != "$latest" ]]; then
    echo "WARN: 'git describe' and tag mismatch: '$revision' vs '$latest', will use '$latest'"
    revision="$latest"
  fi

  if [ -f "$out/$name.json" ] && command -v jq &>/dev/null; then
    current="$(jq -r '.version' "$out/$name.json")"
    echo "Version in existing schema: $current"
    if [[ "$current" == "$revision" ]]; then
      echo "Version in existing schema is same as desired, skipping generation"
      return
    fi
  fi

  sdk="builtin"
  base_file="functions/generate-schema-functions.go"
  echo "Using sdk: $sdk"
  echo "Using base file: $base_file"

  rm -rf generate-schema
  mkdir generate-schema
  sed \
    -e "s~__REPOSITORY__~$repository~g" \
    -e "s~__NAME__~${name}~g" \
    -e "s~__REVISION__~$revision~g" \
    -e "s~__SDK__~$sdk~g" \
    -e "s~__OUT__~$out~g" \
    "$CUR/template/$base_file" \
    >generate-schema/generate-schema.go

	go get "github.com/hashicorp/hil"
  echo "Generating schema for $name"
  if [[ "${GENERATE_PARALLEL:-}" == "1" ]]; then
    (
      generate_one "$name" GO111MODULE=on
    ) &
  else
    generate_one "$name" GO111MODULE=on
  fi

  # Revert to previous state
  [[ -n "$latest" ]] && git checkout --force -q '@{-1}'
  popd >/dev/null || return
}

process_functions "functions"

echo "========================================"
echo "Everything done!"
echo
