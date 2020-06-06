#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null

echo "Checking providers.base.json..."
fail=0
# shellcheck disable=SC2013
for name in $(cat providers.list); do
  if ! jq -re "keys[]" <providers.base.json | grep -q "$name"; then
    echo "New provider avaialble: $name"
    fail=1
  fi
done
if [[ $fail -eq 1 ]]; then
  echo "New providers avaialble, update proviers.base.json"
  exit 1
fi

echo "Done"
popd >/dev/null
