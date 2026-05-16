#!/bin/bash
set -e

# Release script for letsreview
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 0.1.0

if [ -z "$1" ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 0.1.0"
  exit 1
fi

VERSION="$1"

if [[ "$VERSION" == v* ]]; then
  echo "Error: Version should not include 'v' prefix"
  echo "Use: $0 0.1.0"
  echo "Not: $0 v0.1.0"
  exit 1
fi

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: Invalid version format '$VERSION'"
  echo "Version must be in semantic versioning format: X.Y.Z"
  echo "Example: 0.1.0, 1.0.0, 2.3.1"
  exit 1
fi

VERSION_TAG="v$VERSION"
CHANGELOG_FILE="CHANGELOG.md"

echo "Releasing $VERSION_TAG..."

LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

if [[ -n "$LATEST_TAG" ]]; then
  COMMIT_RANGE="$LATEST_TAG..HEAD"
else
  COMMIT_RANGE="HEAD"
fi

REPO_URL=$(git config --get remote.origin.url)
if [[ "$REPO_URL" =~ git@github.com:(.+)\.git$ ]]; then
  REPO_URL="https://github.com/${BASH_REMATCH[1]}"
fi
COMMIT_RANGE_LINK="${REPO_URL}/compare/${LATEST_TAG}...${VERSION_TAG}"

echo "Parsing commits from ${COMMIT_RANGE}..."
echo ""

ADDED=()
FIXED=()
CHANGED=()
OTHER=()

TEMP_COMMITS=$(mktemp)
git log "${COMMIT_RANGE}" --pretty=format:"%s" > "$TEMP_COMMITS"

TEMP_HASHES=$(mktemp)
git log "${COMMIT_RANGE}" --pretty=format:"%s|%H" > "$TEMP_HASHES"

while IFS= read -r commit || [[ -n "$commit" ]]; do
  if [[ "$commit" == *": "* ]]; then
    TYPE="${commit%%:*}"
    MESSAGE="${commit#*: }"
    TYPE="${TYPE%%\(*}"

    case "$TYPE" in
      feat|add)
        ADDED+=("$MESSAGE")
        ;;
      fix|bugfix)
        FIXED+=("$MESSAGE")
        ;;
      chore|refactor|perf|style|test|ci|build|docs)
        CHANGED+=("$MESSAGE")
        ;;
      *)
        OTHER+=("$MESSAGE")
        ;;
    esac
  else
    OTHER+=("$commit")
  fi
done < "$TEMP_COMMITS"

rm -f "$TEMP_COMMITS"

_get_hash_file="$TEMP_HASHES"

get_hash() {
  local msg="$1"
  if [[ -f "$_get_hash_file" ]]; then
    local hash=$(grep -F "$msg" "$_get_hash_file" | cut -d'|' -f2 | head -1)
    echo "$hash"
  else
    echo ""
  fi
}

CHANGELOG_ENTRIES=()

if [[ ${#ADDED[@]} -gt 0 ]]; then
  CHANGELOG_ENTRIES+=("### Added")
  for entry in "${ADDED[@]}"; do
    HASH=$(get_hash "$entry")
    SHORT_HASH="${HASH:0:7}"
    CHANGELOG_ENTRIES+=("- $entry ([${SHORT_HASH}](${REPO_URL}/commit/${HASH}))")
  done
  CHANGELOG_ENTRIES+=("")
fi

if [[ ${#FIXED[@]} -gt 0 ]]; then
  CHANGELOG_ENTRIES+=("### Fixed")
  for entry in "${FIXED[@]}"; do
    HASH=$(get_hash "$entry")
    SHORT_HASH="${HASH:0:7}"
    CHANGELOG_ENTRIES+=("- $entry ([${SHORT_HASH}](${REPO_URL}/commit/${HASH}))")
  done
  CHANGELOG_ENTRIES+=("")
fi

if [[ ${#CHANGED[@]} -gt 0 ]]; then
  CHANGELOG_ENTRIES+=("### Changed")
  for entry in "${CHANGED[@]}"; do
    HASH=$(get_hash "$entry")
    SHORT_HASH="${HASH:0:7}"
    CHANGELOG_ENTRIES+=("- $entry ([${SHORT_HASH}](${REPO_URL}/commit/${HASH}))")
  done
  CHANGELOG_ENTRIES+=("")
fi

if [[ ${#OTHER[@]} -gt 0 ]]; then
  CHANGELOG_ENTRIES+=("<details>")
  CHANGELOG_ENTRIES+=("<summary>Other</summary>")
  CHANGELOG_ENTRIES+=("")
  for entry in "${OTHER[@]}"; do
    HASH=$(get_hash "$entry")
    SHORT_HASH="${HASH:0:7}"
    CHANGELOG_ENTRIES+=("- $entry ([${SHORT_HASH}](${REPO_URL}/commit/${HASH}))")
  done
  CHANGELOG_ENTRIES+=("")
  CHANGELOG_ENTRIES+=("</details>")
  CHANGELOG_ENTRIES+=("")
fi

rm -f "$TEMP_HASHES"

DATE=$(date -u +"%Y-%m-%d")

if [[ ! -f "$CHANGELOG_FILE" ]]; then
  cat > "$CHANGELOG_FILE" << EOF
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

EOF
fi

TEMP_FILE=$(mktemp)
{
  echo "## [$VERSION_TAG] - $DATE"
  echo ""
  if [[ -n "$COMMIT_RANGE_LINK" ]]; then
    echo "[Full Changelog](${COMMIT_RANGE_LINK})"
    echo ""
  fi
  for entry in "${CHANGELOG_ENTRIES[@]}"; do
    echo "$entry"
  done
  echo ""
  cat "$CHANGELOG_FILE"
} > "$TEMP_FILE"
mv "$TEMP_FILE" "$CHANGELOG_FILE"

echo "Changelog updated:"
echo "## [$VERSION_TAG] - $DATE"
for entry in "${CHANGELOG_ENTRIES[@]}"; do
  echo "$entry"
done
echo ""

sed -i.bak "s/var Version = \".*\"/var Version = \"$VERSION_TAG\"/" version.go
rm -f version.go.bak

sed -i.bak "s|mohammed-io/letsreview/cmd/letsreview@v[0-9.]*|mohammed-io/letsreview/cmd/letsreview@$VERSION_TAG|g" README.md
rm -f README.md.bak

git add version.go README.md "$CHANGELOG_FILE"
git commit -m "chore: release $VERSION_TAG"

git tag "$VERSION_TAG"
git push origin main --tags
git push origin "$VERSION_TAG"

echo "Released $VERSION_TAG"
