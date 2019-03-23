#!/bin/bash

pushd "$(dirname $0)" > /dev/null

curl -s https://api.github.com/orgs/terraform-providers/repos?per_page=200 | jq '.[].name' -r | sort > providers.list.full
cat providers.list.full | grep -- '^terraform-provider-' | awk '{print substr($0, 20)}' > providers.list
popd > /dev/null