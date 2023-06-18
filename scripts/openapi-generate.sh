#!/usr/bin/env sh

__current_dir="$( cd -- "$( dirname -- "${0}"; )" &> /dev/null && pwd 2> /dev/null; )";
filepath="$__current_dir/chroma/chromaclient/openapi.json"

tempfile=$(mktemp)
curl -s http://localhost:8000/openapi.json | jq > "$tempfile"

# sed works slightly differently over OSes
case "$OSTYPE" in
  darwin*)
    # macOS
    SED_CMD="sed -i ''"
    ;;
  *)
    # Linux and others
    SED_CMD="sed -i"
    ;;
esac

# Since there are quite a few of these, update them  using `sed`
$SED_CMD 's/"schema": {}/"schema": {"type": "object"}/g' "$tempfile"
$SED_CMD 's/"items": {}/"items": { "type": "object" }/g' "$tempfile"
$SED_CMD -e 's/"title": "Collection Name"/"title": "Collection Name","type": "string"/g' "$tempfile"

# oapi-codegen doesn't like it when the enums are split up in an `anyOf`.
# These are structured like so, kind of:
# ```
# anyOf:
#   - type: string
#     enum: [documents]
#   - type: string
#     enum: [embeddings]
#   ...etc
# ```
# We want them in a single enum instead (type: string, enum: [documents, embeddings])
jq '
  (.components.schemas.QueryEmbedding.properties.include).items |= {
    "type": "string",
    "enum": (.anyOf | map(.enum[0]))
  }
  | (.components.schemas.GetEmbedding.properties.include).items |= {
      "type": "string",
      "enum": (.anyOf | map(.enum[0]))
  }
' "$tempfile" > "$filepath"

echo "Wrote $filepath"
