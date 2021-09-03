#!/usr/bin/env bash

config_file='providers.json'

. common.sh

logs="$CUR/logs/providers"
failures="$CUR/providers-failure.txt"
out="$CUR/schemas/providers"

mkdir -p "$out"
rm -f "$failures"

if [ ! -z ${ONLY+x} ]; then 
  echo "Provider list found. Only passed in providers will be processed."
  IFS=' ' read -r -a CHECK_LIST <<< $ONLY
fi

update_all

echo
echo "========================================"
echo "Waiting for update processes to finish"
wait
echo "All providers updated"
echo

function process_provider() {
  name="$1"
  if [[ "$name" == "__NAME__" ]]; then
    return 0
  fi
  skip_generation="$(jq_get "$name" 'skip_generation')"
  if [[ $skip_generation == "true" ]]; then
    return 0
  fi
  repository="$(jq_get "$name" 'repository')"
  pkg_name="$(jq_get "$name" 'pkg_name')"
  provider_args="$(jq_get "$name" 'provider_args')"
  provider_func_name="$(jq_get "$name" 'provider_func_name')"
  use_master="$(jq_get "$name" 'use_master')"
  go_envs="$(jq_get "$name" 'go_envs')"
  go_args=""
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
    git tag -l --sort=-v:refname | head -n 5
    latest=$(git tag -l --sort=-v:refname | head -n 1)
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

  sdk="terraform"
  if [ -f "go.mod" ] && [ -d 'tfplugin5' ]; then
    sdk="terraform-tfplugin5"
    go_envs="GO111MODULE=on"
    go_args="-mod=mod"
    GO111MODULE=on go get github.com/hashicorp/hcl/v2/ext/customdecode
    GO111MODULE=on go get github.com/hashicorp/hcl/v2
    GO111MODULE=on go get github.com/zclconf/go-cty/cty/json
  elif [ -f "go.mod" ] && grep -q 'github.com/hashicorp/terraform-plugin-sdk/v2' "go.mod"; then
    sdk="terraform-plugin-sdk-v2"
  elif grep -q 'github.com/hashicorp/terraform-plugin-sdk/v2' -r "$pkg_name"; then
    sdk="terraform-plugin-sdk-v2"
  elif [ -f "go.mod" ] && grep -q 'github.com/hashicorp/terraform-plugin-sdk v1' "go.mod"; then
    sdk="terraform-plugin-sdk-v1"
  elif grep -q 'github.com/hashicorp/terraform-plugin-sdk' -r "$pkg_name"; then
    sdk="terraform-plugin-sdk-v1"
  fi

  if [ "$sdk" != "terraform" ] && [ ! -d "vendor" ] && [ -f "go.mod" ]; then
    go_envs="GO111MODULE=on"
  fi
  # TODO: Detect and use terraform-plugin-sdk-v2 when needed
  base_file="provider/$sdk/generate-schema.go"
  echo "Using sdk: $sdk"
  echo "Using base file: $base_file"
  echo "Using go_envs: $go_envs"

  [ -f 'go.mod' ] && cat >>'go.mod' <<'EOF'
replace github.com/go-critic/go-critic v0.0.0-20181204210945-1df300866540 => github.com/go-critic/go-critic v0.3.5-0.20190526074819-1df300866540
replace github.com/golangci/errcheck v0.0.0-20181003203344-ef45e06d44b6 => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
replace github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196
replace github.com/golangci/gofmt v0.0.0-20181105071733-0b8337e80d98 => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98
replace github.com/golangci/gosec v0.0.0-20180901114220-66fb7fc33547 => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547
replace github.com/golangci/ineffassign v0.0.0-20180808204949-42439a7714cc => github.com/golangci/ineffassign v0.0.0-20190609212857-42439a7714cc
replace github.com/golangci/lint-1 v0.0.0-20180610141402-ee948d087217 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217
replace mvdan.cc/unparam v0.0.0-20190124213536-fbb59629db34 => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
EOF

  rm -rf generate-schema
  mkdir generate-schema
  sed \
    -e "s~__REPOSITORY__~$repository~g" \
    -e "s~__NAME__~${name}~g" \
    -e "s~__PKG_NAME__~${pkg_name}~g" \
    -e "s~__REVISION__~$revision~g" \
    -e "s~__PROVIDER_ARGS__~$provider_args~g" \
    -e "s~__PROVIDER_FUNC_NAME__~$provider_func_name~g" \
    -e "s~__SDK__~$sdk~g" \
    -e "s~__OUT__~$out~g" \
    "$CUR/template/$base_file" \
    >generate-schema/generate-schema.go

  echo "Generating schema for $name"
  if [[ "${GENERATE_PARALLEL:-}" == "1" ]]; then
    (
      generate_one "$name" "$go_envs" "$go_args"
    ) &
  else
    generate_one "$name" "$go_envs" "$go_args"
  fi

  # Revert to previous state
  [[ -n "$latest" ]] && git checkout --force -q '@{-1}'
  popd >/dev/null || return
}

while IFS= read -r p; do
  if [ ! -z ${ONLY+x} ]; then 
    if [[ " ${CHECK_LIST[*]} " =~ " ${p} " ]]; then
      process_provider "$p" || true
    fi
  else
    process_provider "$p" || true
  fi
done < <(jq -r 'keys[]' <"$CUR/$config_file")

echo
echo "========================================"
echo "Waiting for 'generate-schemas' processes to finish"
wait
echo

echo "========================================"
echo "Everything done!"
echo
