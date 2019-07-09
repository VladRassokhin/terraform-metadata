#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null

get_pages() {
  local idx=1
  while curl -s "https://api.github.com/orgs/terraform-providers/repos?sort=full_name&per_page=100&page=${idx}" | jq -re '.[].name'; do
    idx=$((idx + 1))
  done
}

get_pages >providers.list.full
grep -- '^terraform-provider-' providers.list.full | awk '{print substr($0, 20)}' >providers.list
popd >/dev/null
