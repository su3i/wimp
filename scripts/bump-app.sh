#!/usr/bin/env bash
# Bumps the control plane and web version everywhere it is written down.
#
#   scripts/bump-app.sh [x.y.z | patch | minor | major]   (default: patch)
#
# The control plane and web share one version. The agent is versioned separately:
# see scripts/bump-agent.sh.
source "$(dirname "$0")/lib.sh"

current=$(read_version helm/values.yaml 'repository: \S+/wimp\n\s*tag: "')
next=$(next_version "$current" "${1:-patch}")
echo "control plane + web: $current -> $next"

changes() {
  set_value .github/workflows/build-control-plane.yml '/wimp:' "$SEMVER" '' "$next"
  set_value .github/workflows/build-web.yaml '/wimp-web:' "$SEMVER" '' "$next"
  set_value helm/values.yaml 'repository: \S+/wimp\n\s*tag: "' '[^"]*' '"' "$next"
  set_value helm/values.yaml 'repository: \S+/wimp-web\n\s*tag: "' '[^"]*' '"' "$next"
  set_value helm/Chart.yaml '^appVersion: "' '[^"]*' '"' "$next"
  set_value web/package.json '^  "version": "' '[^"]*' '"' "$next"
  set_value internal/infrastructure/server/handlers/health.go '"version": +"v' '[^"]*' '"' "$next"
  set_value .env.example '^WIMP_TAG=' '\S*' '' "$next"
  set_value .env.example '^WIMP_WEB_TAG=' '\S*' '' "$next"
  set_value .env '^WIMP_TAG=' '\S*' '' "$next" optional
  set_value .env '^WIMP_WEB_TAG=' '\S*' '' "$next" optional
  bump_chart
}

apply changes
