#!/bin/bash

pushd "$(dirname $0)" > /dev/null

get_pages () {
  local idx=1
  while curl -s "https://api.github.com/orgs/terraform-providers/repos?sort=full_name&per_page=100&page=${idx}" | jq -re '.[].name' ; do
    idx=$((idx + 1))
  done
}

get_pages > providers.list.full
cat providers.list.full | grep -- '^terraform-provider-' | awk '{print substr($0, 20)}' > providers.list
popd > /dev/null
