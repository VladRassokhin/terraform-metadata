#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null

which xmllint &>/dev/null || ( echo "xmllint required"; exit 1 )
which jq &>/dev/null || ( echo "jq required"; exit 1 )
which tr &>/dev/null || ( echo "tr required"; exit 1 )

function urldecode() { : "${*//+/ }"; echo -e "${_//%/\\x}"; }

rm -f providers.html
curl 'https://registry.terraform.io/search/providers' -o providers.html
environment="$(xmllint --html -xpath 'string(//meta[@name="terraform-registry/config/environment"]/@content)' providers.html 2>/dev/null)"
decoded_environment="$(urldecode "$environment")"
rm providers.html

APP_ID="$(echo "$decoded_environment" | jq .algolia.APP_ID -r)"
API_KEY="$(echo "$decoded_environment" | jq .algolia.API_KEY -r)"
index="$(echo "$decoded_environment" | jq .algolia.indices.providers -r)"

function get_providers() {
  type="$1" # one of: official, partner
  file="$2"
  curl -X POST "https://${APP_ID}-3.algolianet.com/1/indexes/${index}/query" \
    -H "x-algolia-application-id: ${APP_ID}" \
    -H "x-algolia-api-key: ${API_KEY}" \
    -H "User-Agent: curl terraform-metadata-crawler" \
    -H 'content-type: application/x-www-form-urlencoded' \
    --data-raw "{\"page\":0,\"hitsPerPage\":500,\"facetFilters\":[[\"tier:${type}\"]],\"getRankingInfo\":false}" \
    -o "$file" --silent --fail
}

get_providers "official" "dump.official.json"
get_providers "partner" "dump.partner.json"

function process_dump() {
  type="$1"
  # jq '.hits[] | [."full-name", .source] | @tsv' -r "dump.$type.json" | tr '\t' ',' > "providers.$type.csv"
  jq '.hits[] | {(."name") : {"full-name":."full-name", "repository":.source, "latest-version":."latest-version"}}'  -r "dump.$type.json"  | jq -n '[inputs] | add' > "providers.$type.json"
  rm "dump.$type.json"
}

process_dump "official"
process_dump "partner"

jq -s '.[0] * .[1]' "providers.official.json" "providers.partner.json" > "providers.registry.json"
