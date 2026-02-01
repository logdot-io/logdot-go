#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BUMP=false

for arg in "$@"; do
  case "$arg" in
    --bump) BUMP=true ;;
    *)
      echo "Usage: $0 [--bump]"
      echo "  --bump  Bump patch version before publishing"
      exit 1
      ;;
  esac
done

# Get the latest version tag
LATEST_TAG=$(git describe --tags --abbrev=0 --match "v*" 2>/dev/null || echo "v0.0.0")
CURRENT_VERSION="${LATEST_TAG#v}"

if [ "$BUMP" = true ]; then
  IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"
  NEW_PATCH=$((PATCH + 1))
  NEW_VERSION="${MAJOR}.${MINOR}.${NEW_PATCH}"
  TAG="v${NEW_VERSION}"

  echo "Bumping version: v${CURRENT_VERSION} -> ${TAG}"
else
  NEW_VERSION="$CURRENT_VERSION"
  TAG="v${NEW_VERSION}"
  echo "Publishing ${TAG}..."
fi

echo "Running tests..."
go test ./...

echo "Running vet..."
go vet ./...

if [ "$BUMP" = true ]; then
  echo "Creating tag ${TAG}..."
  git tag -a "$TAG" -m "Release ${TAG}"
fi

echo "Pushing tag ${TAG}..."
git push origin "$TAG"

echo "Successfully published github.com/logdot-io/logdot-go ${TAG}"
echo "Module will be available on pkg.go.dev shortly."
