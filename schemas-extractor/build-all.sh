#!/usr/bin/env bash

if [[ ! -f 'providers.list.full' ]]; then
  echo "File 'providers.list.full' required to operate"
  exit 1
fi
if [[ -z "$GOPATH" ]]; then
  echo 'GOPATH env variable required to operate'
  exit 1
fi

CUR="$(pwd)"

out="$CUR/schemas"
mkdir -p "$out"
rm -f "$CUR/failure.txt"

mkdir -p "$GOPATH/src/github.com/terraform-providers"

update_or_clone() {
  name="$1"
  location="$GOPATH/src/github.com/terraform-providers/$name"
  if [[ -d "$location" ]]; then
    echo "Updating $name"
    [[ -d "$location/vendor" ]] && git -C "$location" checkout -- 'vendor/' >/dev/null
    git -C "$location" pull >/dev/null
  else
    echo "Cloning $name"
    git clone "https://github.com/terraform-providers/$name.git" "$location" >/dev/null
  fi
}

for p in $(cat "$CUR/providers.list.full"); do
  update_or_clone "$p" &
done

pushd "$GOPATH/src/github.com/terraform-providers" >/dev/null

echo
echo "========================================"
echo "Waiting for update processes to finish"
wait
echo "All providers updated"
echo

generate_one() {
  go run generate-schema/generate-schema.go
  if [[ $? -ne 0 ]]; then
     echo "$1" >> "$2/failure.txt"
  fi
  echo "Finished $1"
}

for p in $(cat "$CUR/providers.list.full"); do
  if [[ "$p" == "terraform-provider-scaffolding" ]]; then
    continue
  fi

  pushd "$p" >/dev/null

  echo "Preparing $p"
  revision="$(git describe)"

  name="${p#terraform-provider-}"
  pkg_name="$name"
  if [[ "$pkg_name" == "azure-classic" ]]; then
    pkg_name="azure"
  fi

  rm -rf generate-schema
  mkdir generate-schema
  cp -r "$CUR/template/generate-schema.go" generate-schema/generate-schema.go
  sed -i -e "s/__FULL_NAME__/$p/g" generate-schema/generate-schema.go
  sed -i -e "s/__NAME__/${name}/g" generate-schema/generate-schema.go
  sed -i -e "s/__PKG_NAME__/${pkg_name}/g" generate-schema/generate-schema.go
  sed -i -e "s/__REVISION__/$revision/g" generate-schema/generate-schema.go
  sed -i -e "s~__OUT__~$out~g" generate-schema/generate-schema.go

  #echo "Building $p"
  #make

  echo "Generating schema for $p"
  if [[ "$KILL_CPU" == "1" ]]; then
  (
    generate_one "$p" "$CUR"
  )&
  else
    generate_one "$p" "$CUR"
  fi

  popd >/dev/null
done

if [[ "$KILL_CPU" == "1" ]]; then
echo
echo "========================================"
echo "Waiting for 'generate-schemas' processes to finish"
wait
echo
fi

echo "========================================"
echo "Everything done!"
echo

popd >/dev/null

