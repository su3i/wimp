# Shared by the bump-*.sh scripts. Sourced, not run.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEMVER='[0-9]+\.[0-9]+\.[0-9]+'

die() { echo "error: $*" >&2; exit 1; }

# read_version FILE PREFIX: the semver that follows the perl regex PREFIX in FILE.
read_version() {
  local v
  v=$(perl -0ne 'print $1 if m{'"$2"'('"$SEMVER"')}m' "$ROOT/$1")
  [[ -n $v ]] || die "no version found after '$2' in $1"
  echo "$v"
}

# next_version CURRENT ARG: ARG is an explicit x.y.z, or patch|minor|major to increment CURRENT.
next_version() {
  local major minor patch
  IFS=. read -r major minor patch <<<"$1"
  case "$2" in
    patch) echo "$major.$minor.$((patch + 1))" ;;
    minor) echo "$major.$((minor + 1)).0" ;;
    major) echo "$((major + 1)).0.0" ;;
    *) [[ $2 =~ ^$SEMVER$ ]] || die "'$2' is not x.y.z, patch, minor or major"; echo "$2" ;;
  esac
}

# set_value FILE PREFIX OLD SUFFIX VALUE [optional]
# Replaces every PREFIX OLD SUFFIX (perl regexes) in FILE with PREFIX VALUE SUFFIX. Fails when
# nothing matches, so a version string that moved is noticed rather than silently skipped.
# With "optional", a missing FILE is skipped (for untracked local files like .env).
set_value() {
  local file=$1 prefix=$2 old=$3 suffix=$4 value=$5 optional=${6:-}
  if [[ ! -f $ROOT/$file ]]; then
    [[ -n $optional ]] && return 0
    die "$file not found"
  fi
  local hits
  hits=$(perl -0ne '$n++ while m{('"$prefix"')'"$old"'('"$suffix"')}gm; print $n+0' "$ROOT/$file")
  if [[ $hits -eq 0 ]]; then
    [[ -n $optional ]] && return 0
    die "pattern '$prefix$old$suffix' not found in $file, nothing was changed"
  fi
  [[ -n ${CHECK_ONLY:-} ]] && return 0
  VALUE="$value" perl -0pi -e 's{('"$prefix"')'"$old"'('"$suffix"')}{$1$ENV{VALUE}$2}gm' "$ROOT/$file"
  echo "  $file ($hits)"
}

# bump_chart: every change under helm/ republishes the chart, and the registry needs a new
# chart version each time, so both bump scripts end with this.
bump_chart() {
  local current next
  current=$(read_version helm/Chart.yaml '^version: ')
  next=$(next_version "$current" patch)
  set_value helm/Chart.yaml '^version: ' "$SEMVER" '' "$next"
  [[ -n ${CHECK_ONLY:-} ]] || echo "  chart version $current -> $next"
}

# apply FN: runs FN once checking every location, then again writing, so a location that no
# longer matches stops the bump before any file is touched.
apply() {
  CHECK_ONLY=1 "$1"
  CHECK_ONLY= "$1"
}
