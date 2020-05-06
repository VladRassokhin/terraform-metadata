#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null

get_pages() {
  local idx=1
  while curl -s "https://api.github.com/orgs/terraform-providers/repos?sort=full_name&per_page=100&page=${idx}" | jq -re '.[].name'; do
    echo "Fetched page $idx" 1>&2
    idx=$((idx + 1))
  done
  local idx=1
  while curl -s "https://api.github.com/orgs/hashicorp/repos?sort=full_name&per_page=100&page=${idx}" | jq -re '.[].name'; do
    echo "Fetched page $idx" 1>&2
    idx=$((idx + 1))
  done
}

get_pages >providers.list.full
grep -- '^terraform-provider-' providers.list.full | awk '{print substr($0, 20)}' | sort >providers.list
popd >/dev/null
