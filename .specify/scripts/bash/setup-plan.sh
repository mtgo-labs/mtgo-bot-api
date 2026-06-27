#!/usr/bin/env bash
# Detects the current feature spec (latest spec under specs/) and prepares the plan.
set -euo pipefail

SPECS_DIR="specs"

if [[ ! -d "${SPECS_DIR}" ]]; then
  echo "Error: no specs/ directory found" >&2
  exit 1
fi

# Find the most recently modified spec directory containing a spec.md
FEATURE_DIR=""
for d in "${SPECS_DIR}"/*/; do
  if [[ -f "${d}spec.md" ]]; then
    FEATURE_DIR="${d%/}"
  fi
done

if [[ -z "${FEATURE_DIR}" ]]; then
  echo "Error: no spec.md found under ${SPECS_DIR}/" >&2
  exit 1
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
FEATURE_SPEC="${FEATURE_DIR}/spec.md"
IMPL_PLAN="${FEATURE_DIR}/plan.md"

# Copy plan template if no plan exists yet.
if [[ ! -f "${IMPL_PLAN}" && -f ".specify/templates/plan-template.md" ]]; then
  cp ".specify/templates/plan-template.md" "${IMPL_PLAN}"
fi

cat <<EOF
{
  "feature_dir": "${FEATURE_DIR}",
  "feature_spec": "${FEATURE_SPEC}",
  "impl_plan": "${IMPL_PLAN}",
  "specs_dir": "${SPECS_DIR}",
  "branch": "${BRANCH}"
}
EOF
