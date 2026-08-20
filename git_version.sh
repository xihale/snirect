#!/bin/bash
# versionName = <tag>[-<n>-g<hash>][-dirty]
# versionCode is git rev-list --count HEAD (see android/app/build.gradle).

# 1. Find the closest tag
TAG=$(git describe --tags --abbrev=0 2>/dev/null)

if [ -z "$TAG" ]; then
    TAG="0.0.0"
fi

# 2. Get current commit hash
HASH=$(git rev-parse --short HEAD 2>/dev/null)

if [ -z "$HASH" ]; then
    echo "0.0.0-unknown"
    exit 0
fi

# 3. Calculate distance (number of commits since tag)
if [ "$TAG" != "0.0.0" ]; then
    DISTANCE=$(git rev-list --count ${TAG}..HEAD 2>/dev/null)
else
    DISTANCE=$(git rev-list --count HEAD 2>/dev/null)
fi

# 4. Check for uncommitted changes (dirty state)
if [ -n "$(git status --porcelain)" ]; then
    DIRTY="-dirty"
else
    DIRTY=""
fi

# 5. Construct version string
if [ "$DISTANCE" -eq "0" ]; then
    # Exact tag match
    VERSION="${TAG}${DIRTY}"
else
    # Tag + distance + hash
    VERSION="${TAG}-${DISTANCE}-g${HASH}${DIRTY}"
fi

echo "$VERSION"
