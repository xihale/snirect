#!/bin/bash
# versionName = <tag>[-<n>-g<hash>][-dirty]
# versionCode is git rev-list --count HEAD (see android/app/build.gradle).
#
# 第一个参数是 tag 匹配模式 (git describe --match): CLI 用 "v*", Android 用
# "android-v*"。两条发版线共用同一仓库, 不带 --match 会挑到任意系列的最近
# tag, 版本号互相污染。

PATTERN="${1:-v*}"

# 1. Find the closest tag
TAG=$(git describe --tags --abbrev=0 --match "$PATTERN" 2>/dev/null)

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
