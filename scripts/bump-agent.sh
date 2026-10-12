#!/usr/bin/env bash
# Bumps the agent version everywhere it is written down.
#
#   scripts/bump-agent.sh [x.y.z | patch | minor | major]   (default: patch)
#
# The release workflow builds agent.exe at this version, and the control plane tells every
# host to run it, so both must agree.
source "$(dirname "$0")/lib.sh"

current=$(read_version .github/workflows/build-agent.yml 'VERSION: "')
next=$(next_version "$current" "${1:-patch}")
echo "agent: $current -> $next"

changes() {
  set_value .github/workflows/build-agent.yml 'VERSION: "' '[^"]*' '"' "$next"
  set_value helm/values.yaml 'agentVersion: "' '[^"]*' '"' "$next"
  set_value .env.example '^AGENTVERSION=' '\S*' '' "$next"
  set_value .env '^AGENTVERSION=' '\S*' '' "$next" optional
  bump_chart
}

apply changes
