#!/usr/bin/env bash
# Cut a release: gate, tag, push, then point the tap at the new tarball.
#
# Doing this by hand is a nine-step sequence and v0.1.9 lost one of them, so
# every step that can fail gets its own message and nothing is attempted after
# a failure. The tag push is the point of no return: everything that can be
# checked is checked before it, and everything after it is mechanical.
#
# Maintainer tool — English only, unlike every user-facing string in this repo.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

REPO=ShiGaChenTW/ghostty_config_studio
TAP=/opt/homebrew/Library/Taps/shigachentw/homebrew-tap
TAP_FORMULA="$TAP/Formula/ghostty-config-studio.rb"
TEMPLATE=Formula/ghostty-config-studio.rb

die() { echo "release: $*" >&2; exit 1; }
step() { printf '\n==> %s\n' "$1"; }

version="${1:-}"
[ -n "$version" ] || die "usage: scripts/release.sh <version>   (e.g. 0.2.0, no leading v)"
case "$version" in
  v*) die "drop the leading v: pass '${version#v}' — the tag gets the v, the formula does not" ;;
esac
echo "$version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z][0-9A-Za-z.]*)?$' \
  || die "'$version' does not look like a version — want 1.2.3 or 1.2.3-rc1"
tag="v$version"

step "pre-flight"
branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || die "on branch '$branch', releases are cut from main"
[ -z "$(git status --porcelain)" ] \
  || die "working tree is dirty — commit, stash or ignore these first:
$(git status --short | sed 's/^/    /')"
! git rev-parse -q --verify "refs/tags/$tag" >/dev/null \
  || die "tag $tag already exists locally — 'git tag -d $tag' if it was a false start"
! git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1 \
  || die "tag $tag is already on the remote — $version is published, pick the next version"
[ -f "$TAP_FORMULA" ] || die "tap formula not found at $TAP_FORMULA — is the tap installed?"
[ -z "$(git -C "$TAP" status --porcelain)" ] || die "tap working tree is dirty: $TAP"

# The tap formula is the one that actually builds. If it has drifted from the
# template in this repo by anything other than the two lines this script
# rewrites, the release would ship the old build recipe under the new version.
if ! diff -q <(grep -vE '^  (url|sha256) ' "$TEMPLATE") \
             <(grep -vE '^  (url|sha256) ' "$TAP_FORMULA") >/dev/null; then
  die "tap formula differs from $TEMPLATE beyond url/sha256.
    review:  diff $TEMPLATE $TAP_FORMULA
    accept:  cp $TEMPLATE $TAP_FORMULA   (then re-run; url/sha256 are rewritten below)"
fi
echo "    $tag from main, tap in sync"

step "checks"
./scripts/check.sh || die "scripts/check.sh failed"
./scripts/test-writes.sh >/dev/null || die "scripts/test-writes.sh failed — run it directly to see which assertion"
# Not in test-writes.sh: it costs ~20s of probing the Ghostty binary, and it
# checks this build against whatever Ghostty happens to be installed rather
# than against the code. A release is exactly when that is worth knowing.
./scripts/check-catalog.sh || die "tui/keycatalog.go has drifted from the installed Ghostty (see above)"
(cd tui && go build ./...) || die "go build failed"
(cd tui && go vet ./...) || die "go vet failed"

step "tagging $tag"
git tag -a "$tag" -m "$tag"
git push origin main || die "pushing main failed — 'git tag -d $tag' and retry"
git push origin "$tag" || die "pushing $tag failed — 'git tag -d $tag' and retry"

step "waiting for the release tarball"
# GitHub builds the archive on demand; it 404s for a second or two after the
# tag lands, which is not a reason to abandon a release that is already pushed.
url="https://github.com/$REPO/archive/refs/tags/$tag.tar.gz"
tarball=$(mktemp -t ghostty-release)
trap 'rm -f "$tarball"' EXIT
for attempt in 1 2 3 4 5 6 7 8; do
  # -sS is deliberately not used: a 404 here is the expected case, and curl's
  # own error on every retry would bury the one message that matters.
  if curl -fsL -o "$tarball" "$url"; then break; fi
  [ "$attempt" -lt 8 ] || die "$url still unavailable after 8 tries.
    the tag is pushed, so re-running is not safe — finish by hand:
      shasum -a 256 <(curl -fsSL $url)
      then edit url + sha256 in $TAP_FORMULA"
  echo "    not ready yet (attempt $attempt), retrying in 3s"
  sleep 3
done
sha=$(shasum -a 256 "$tarball" | awk '{print $1}')
echo "    sha256 $sha"

step "updating the tap"
sed -i '' \
  -e "s|^  url \".*\"|  url \"$url\"|" \
  -e "s|^  sha256 \".*\"|  sha256 \"$sha\"|" \
  "$TAP_FORMULA"
grep -q "^  url \"$url\"$" "$TAP_FORMULA" || die "url rewrite did not take in $TAP_FORMULA"
grep -q "^  sha256 \"$sha\"$" "$TAP_FORMULA" || die "sha256 rewrite did not take in $TAP_FORMULA"
git -C "$TAP" add Formula/ghostty-config-studio.rb
git -C "$TAP" commit -q -m "ghostty-config-studio $version"
git -C "$TAP" push -q || die "tap commit is made but the push failed — 'git -C $TAP push' by hand"

# Installing is deliberately left to you. `brew upgrade` mutates the machine
# this script is running on and can pull in unrelated formulae; publishing and
# installing are separate decisions, and only the first one is a release.
step "$tag published — install it with"
cat <<EOS
    brew update && brew upgrade ghostty-config-studio
    brew test ghostty-config-studio
    ghostty-tui --version        # must print: ghostty-config-studio $version
EOS
