#!/usr/bin/env bash
set -euo pipefail

# cd to the project root (parent of scripts/)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

URL_LIST="maxmind-mmdb-urls.txt"

if [ ! -f "$URL_LIST" ]; then
    echo "ERROR: ${URL_LIST} not found in ${PROJECT_ROOT}" >&2
    exit 1
fi

while IFS= read -r url; do
    # skip blank lines and comments
    [[ -z "$url" || "$url" =~ ^[[:space:]]*# ]] && continue

    filename="$(basename "$url")"
    tmpfile="${filename}.tmp"
    echo "Downloading ${filename} ..."
    curl --fail --location --silent --show-error --output "$tmpfile" "$url"
    mv "$tmpfile" "$filename"
done < "$URL_LIST"

echo "All MMDB files fetched successfully."
