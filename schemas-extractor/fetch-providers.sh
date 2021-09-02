#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null

get_repos() {
  echo "Fetching repositories from organization $1..."
  local idx=1
  while curl -s "https://api.github.com/orgs/$1/repos?sort=full_name&per_page=100&page=${idx}" | jq -re '.[].name' >>"$2"; do
    echo "Fetched page $idx"
    idx=$((idx + 1))
  done
  echo "Done"
}

rm -f repositories.list.full
get_repos "terraform-providers" repositories.list.full
get_repos "hashicorp" repositories.list.full
echo "Filtering terraform providers repos..."
grep -- '^terraform-provider-' repositories.list.full | awk '{print substr($0, 20)}' | sort >providers.list

echo "Done"
popd >/dev/null
