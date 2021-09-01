#!/usr/bin/env bash

config_file='providers.json'

. common.sh

logs="$CUR/logs/providers"
failures="$CUR/providers-failure.txt"
out="$CUR/schemas/providers"

mkdir -p "$out"
rm -f "$failures"


function process_provider() {
  name="$1"
  if [[ "$name" == "__NAME__" ]]; then
    return 0
  fi
  skip_generation="$(jq_get "$name" 'skip_generation')"
  if [[ $skip_generation == "true" ]]; then
    return 0
  fi
  if [[ -f "$out/$name.json" ]]; then
    echo "Skipping, already generated $name"
    return 0
  fi
  src="$(jq_get "$name" 'source')"
  location=$(mktemp -d "terraform-export-schema-XXXXXX")

  pushd "$location" >/dev/null || return

  echo "Preparing $name"

  latest="latest"
  #echo "Will generate schema for version '$latest'"

  cat >main.tf <<EOF
provider "$name" {}
terraform {
  required_providers {
    $name = {
      source = "$src/$name"
#      version = "$latest"
    }
  }
}
EOF
  terraform init || exit 1
  echo "Generating schema for $name"
  terraform providers schema -json > "$out/$name.json"
  popd >/dev/null || return
  rm -rf "$location"
}

while IFS= read -r p; do
  process_provider "$p" || true
done < <(jq -r 'keys[]' <"$CUR/$config_file")

echo
echo "========================================"
echo "Waiting for 'generate-schemas' processes to finish"
wait
echo

echo "========================================"
echo "Everything done!"
echo
