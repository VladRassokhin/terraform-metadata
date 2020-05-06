#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

if [[ ! -f 'providers.json' ]]; then
  echo "File 'providers.json' required to operate"
  exit 1
fi
if [[ -z "$GOPATH" ]]; then
  echo 'GOPATH env variable required to operate'
  exit 1
fi

CUR="$(pwd)"

out="$CUR/schemas"
mkdir -p "$out"
rm -f "$CUR/failure.txt"

mkdir -p "$GOPATH/src/github.com/terraform-providers"

function jq_get() {
  name="$1"
  prop="$2"
  jq -r ".\"$name\".$prop // .__NAME__.$prop " <"$CUR/providers.json" | sed -e "s/__NAME__/$name/"
}

update_or_clone() {
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

if [[ -z "${SKIP_UPDATE:-}" ]]; then
  while IFS= read -r p; do
    update_or_clone "$p" &
  done < <(jq -r 'keys[]' <"$CUR/providers.json")
fi

pushd "$GOPATH/src/github.com/terraform-providers" >/dev/null

sleep 1 # fix logs
echo
echo "========================================"
echo "Waiting for update processes to finish"
wait
echo "All providers updated"
echo

generate_one() {
  mkdir -p "$2/logs"
  set +e
  go run generate-schema/generate-schema.go 2>&1 | tee "$2/logs/$1.log"
  ec=$?
  if [[ $ec -eq 0 ]]; then
    rm "$2/logs/$1.log"
  else
    echo "$1" >>"$2/failure.txt"
  fi
  echo "Finished $1"
  set -e
}

process_provider() {
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
  location="$GOPATH/src/$repository"

  if [[ ! -d "$location" ]]; then
    echo "$location doesn't exist, skipping"
    return 1
  fi

  if output=$(git -C "$location" status --untracked-files=no --porcelain) && [[ -n "$output" ]]; then
    echo "git working copy '$location' is not clear, cannot proceed"
    echo "$name" >>"$CUR/failure.txt"
    return 2
  fi

  pushd "$location" >/dev/null

  echo "Preparing $name"

  # All tags:
  echo "Repository newest tags:"
  git tag -l --sort=-v:refname | head
  latest=$(git tag -l --sort=-v:refname | head -n 1)
  if [[ -z "$latest" ]]; then
    echo "There's no tags in $name, will use current state"
  else
    echo "Will generate schema for tag '$latest'"
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
  if [ -f "go.mod" ] && grep -q 'github.com/hashicorp/terraform-plugin-sdk' "go.mod"; then
    sdk="terraform-plugin-sdk"
  fi

  cat >>'go.mod' <<'EOF'
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
    -e "s/__NAME__/${name}/g" \
    -e "s/__PKG_NAME__/${pkg_name}/g" \
    -e "s/__REVISION__/$revision/g" \
    -e "s/__PROVIDER_ARGS__/$provider_args/g" \
    -e "s/__SDK__/$sdk/g" \
    -e "s~__OUT__~$out~g" \
    "$CUR/template/generate-schema.go" \
    >generate-schema/generate-schema.go

  echo "Generating schema for $name"
  if [[ "${KILL_CPU:-}" == "1" ]]; then
    (
      generate_one "$name" "$CUR"
    ) &
  else
    generate_one "$name" "$CUR"
  fi

  # Revert to previous state
  [[ -n "$latest" ]] && git checkout --force -q '@{-1}'
  popd >/dev/null
}

while IFS= read -r p; do
  process_provider "$p" || true
done < <(jq -r 'keys[]' <"$CUR/providers.json")

echo
echo "========================================"
echo "Waiting for 'generate-schemas' processes to finish"
wait
echo

echo "========================================"
echo "Everything done!"
echo

popd >/dev/null
