#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

pushd "$(dirname "$0")" >/dev/null
cd ../terraform/model
ls providers | awk '{print substr($0, 1, length($0)-5)}' | sort > providers.list
popd >/dev/null
