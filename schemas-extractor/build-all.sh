#!/usr/bin/env bash

if [[ ! -f 'providers.list.full' ]]; then
  echo "File 'providers.list.full' required to operate"
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

update_or_clone() {
  name="$1"
  location="$GOPATH/src/github.com/terraform-providers/$name"
  if [[ -d "$location" ]]; then
    echo "Updating $name"
    git -C "$location" pull >/dev/null || echo "ERROR: Failed to update '$name'"
  else
    echo "Cloning $name"
    git clone --quiet "https://github.com/terraform-providers/$name.git" "$location" >/dev/null
  fi
}

[[ -z "$SKIP_UPDATE" ]] && for p in $(cat "$CUR/providers.list.full"); do
  update_or_clone "$p" &
done

pushd "$GOPATH/src/github.com/terraform-providers" >/dev/null

echo
echo "========================================"
echo "Waiting for update processes to finish"
wait
echo "All providers updated"
echo

generate_one() {
  go run generate-schema/generate-schema.go
  if [[ $? -ne 0 ]]; then
     echo "$1" >> "$2/failure.txt"
  fi
  echo "Finished $1"
}

process_repository() {
  full_name="$1"
  location="$GOPATH/src/github.com/terraform-providers/$full_name"
  name="${full_name#terraform-provider-}"
  pkg_name="$name"
  provider_args=""

  if [[ "$pkg_name" == "azure-classic" ]]; then
    pkg_name="azure"
  fi
  if [[ "$pkg_name" == "oci" ]]; then
    provider_args="prvdr.ProviderConfig"
  fi

  if output=$(git -C "$location" status --untracked-files=no --porcelain) && [[ ! -z "$output" ]]; then
    echo "git working copy is not clear, cannot proceed"
    echo "$full_name" >> "$CUR/failure.txt"
    exit 2
  fi

  pushd "$location" >/dev/null

  echo "Preparing $full_name"

  # All tags:
  echo "Repository all tags:"
  echo "$(git tag -l --sort=-v:refname | (head; tail))"
  latest=$(git tag -l --sort=-v:refname | head -n 1)
  if [[ -z "$latest" ]]; then
    echo "There's no tags in $full_name, will use current state"
  else
    echo "Will generate schema for tag '$latest'"
  fi

  [[ -n "$latest" ]] && git checkout -q "$latest"


  revision="$(git describe --tags)"
  if [[ -n "$latest" ]] && [[ "$revision" != "$latest" ]]; then
    echo "WARN: 'git describe' and tag mismatch: '$revision' vs '$latest', will use '$latest'"
    revision="$latest"
  fi

  rm -rf generate-schema
  mkdir generate-schema
  cp -r "$CUR/template/generate-schema.go" generate-schema/generate-schema.go
  sed -i -e "s/__FULL_NAME__/$full_name/g" generate-schema/generate-schema.go
  sed -i -e "s/__NAME__/${name}/g" generate-schema/generate-schema.go
  sed -i -e "s/__PKG_NAME__/${pkg_name}/g" generate-schema/generate-schema.go
  sed -i -e "s/__REVISION__/$revision/g" generate-schema/generate-schema.go
  sed -i -e "s/__PROVIDER_ARGS__/$provider_args/g" generate-schema/generate-schema.go
  sed -i -e "s~__OUT__~$out~g" generate-schema/generate-schema.go

  echo "Generating schema for $full_name"
  if [[ "$KILL_CPU" == "1" ]]; then
  (
    generate_one "$full_name" "$CUR"
  )&
  else
    generate_one "$full_name" "$CUR"
  fi

  # Revert to previous state
  [[ -n "$latest" ]] && git checkout -q '@{-1}'
  popd >/dev/null
}

for p in $(cat "$CUR/providers.list.full"); do
  if [[ "$p" == "terraform-provider-scaffolding" ]]; then
    continue
  fi

  process_repository "$p"
done

echo
echo "========================================"
echo "Waiting for 'generate-schemas' processes to finish"
wait
echo

echo "========================================"
echo "Everything done!"
echo

popd >/dev/null

