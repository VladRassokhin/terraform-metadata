#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

if [[ -n "${CHECK_SKIP:-}" ]]; then
  echo "providers.base.json check skipped"
  exit 0
fi


pushd "$(dirname "$0")" >/dev/null

echo "Checking providers.base.json..."
fail=0
conf="providers.registry.json"
rm -f 'providers.base.new.json'
for name in $(jq -r "keys[]" <$conf); do
  if ! jq -re "keys[]" <providers.base.json | grep -q "^$name$"; then
    echo "New provider available: $name"
    {
      echo -n "\"$name\": "
      jq -r ".\"$name\" | del(.\"latest-version\")| del(.\"full-name\")" "$conf"
      echo ","
    } >>'providers.base.new.json'
    fail=1
  fi
done
if [[ $fail -eq 1 ]]; then
  echo "New providers available, update providers.base.json, check providers.base.new.json"
  exit 1
fi

echo "Done"
popd >/dev/null
