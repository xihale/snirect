#!/bin/bash
# Sync the AUR packages (snirect, snirect-bin) to a released version.
# Called by the Release workflow once the release assets are uploaded.
#
# Usage: aur-update.sh <version> [aur-repo ...]   # version without leading v
# Env:  AUR_PUSH=0        dry run: edit + commit, never push
#       GITHUB_REPOSITORY upstream slug (default xihale/snirect)
set -euo pipefail

ver="${1:?usage: aur-update.sh <version-without-v> [aur-repo ...]}"
shift || true
[ $# -gt 0 ] || set -- snirect snirect-bin

repo_slug="${GITHUB_REPOSITORY:-xihale/snirect}"
base="https://github.com/${repo_slug}"

export GIT_AUTHOR_NAME="xihale" GIT_AUTHOR_EMAIL="i@xihale.top"
export GIT_COMMITTER_NAME="xihale" GIT_COMMITTER_EMAIL="i@xihale.top"

sha() { sha256sum "$1" | cut -d' ' -f1; }

# Download to disk with retries so a truncated stream fails loudly instead of
# producing a silently wrong checksum (curl exit code is checked by set -e).
fetch() { curl -fsSL --retry 3 --retry-all-errors "$1" -o "$2"; }

work="$(mktemp -d)"
chmod 755 "$work" # traversable for the unprivileged makepkg run under CI root
trap 'rm -rf "$work"' EXIT

for repo in "$@"; do
  # Read via https (no key needed), push via ssh so only writes need auth.
  git clone -q "https://aur.archlinux.org/${repo}.git" "${work}/${repo}"
  git -C "${work}/${repo}" remote set-url --push origin "ssh://aur@aur.archlinux.org/${repo}.git"
  cd "${work}/${repo}"

  sed -i "s/^pkgver=.*/pkgver=${ver}/" PKGBUILD
  sed -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD

  if [ "$repo" = "snirect" ]; then
    fetch "${base}/archive/refs/tags/v${ver}.tar.gz" "${work}/src.tar.gz"
    sed -i "s/^sha256sums=('.*')/sha256sums=('$(sha "${work}/src.tar.gz")')/" PKGBUILD
  else
    fetch "https://raw.githubusercontent.com/${repo_slug}/v${ver}/LICENSE" "${work}/LICENSE"
    fetch "${base}/releases/download/v${ver}/snirect-linux-amd64" "${work}/amd64"
    fetch "${base}/releases/download/v${ver}/snirect-linux-arm64" "${work}/arm64"
    sed -i "s/^sha256sums=('.*')/sha256sums=('$(sha "${work}/LICENSE")')/" PKGBUILD
    sed -i "s/^sha256sums_x86_64=('.*')/sha256sums_x86_64=('$(sha "${work}/amd64")')/" PKGBUILD
    sed -i "s/^sha256sums_aarch64=('.*')/sha256sums_aarch64=('$(sha "${work}/arm64")')/" PKGBUILD
  fi

  # makepkg refuses to run as root (CI container). Hand the tree to an
  # unprivileged user for the run, then take it back for the git ops.
  if [ "$(id -u)" -eq 0 ]; then
    useradd -m builduser 2>/dev/null || true
    chown -R builduser .
    runuser -u builduser -- makepkg --printsrcinfo > .SRCINFO
    chown -R root .
  else
    makepkg --printsrcinfo > .SRCINFO
  fi

  if git diff --quiet; then
    echo "aur/${repo}: already at ${ver}, nothing to do"
  else
    git add PKGBUILD .SRCINFO
    git commit -qm "Update to ${ver}"
    if [ "${AUR_PUSH:-1}" = "1" ]; then
      git push -q
      echo "aur/${repo}: updated to ${ver}"
    else
      echo "aur/${repo}: AUR_PUSH=0, keeping local commit"
      git --no-pager show --stat --oneline HEAD | head -5
      git --no-pager diff HEAD~1 -- PKGBUILD
    fi
  fi
done
