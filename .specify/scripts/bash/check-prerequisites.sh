#!/usr/bin/env bash
# Reports the current feature dir and the design documents available for tasks.
set -euo pipefail

SPECS_DIR="specs"
FEATURE_DIR=""

for d in "${SPECS_DIR}"/*/; do
  if [[ -f "${d}plan.md" ]]; then
    FEATURE_DIR="${d%/}"
  fi
done

# Fall back to spec.md if no plan yet.
if [[ -z "${FEATURE_DIR}" ]]; then
  for d in "${SPECS_DIR}"/*/; do
    if [[ -f "${d}spec.md" ]]; then
      FEATURE_DIR="${d%/}"
    fi
  done
fi

if [[ -z "${FEATURE_DIR}" ]]; then
  echo "Error: no feature with plan.md or spec.md under ${SPECS_DIR}/" >&2
  exit 1
fi

AVAILABLE_DOCS=()
for doc in spec.md plan.md research.md data-model.md quickstart.md tasks.md; do
  if [[ -f "${FEATURE_DIR}/${doc}" ]]; then
    AVAILABLE_DOCS+=("${FEATURE_DIR}/${doc}")
  fi
done
if [[ -d "${FEATURE_DIR}/contracts" ]]; then
  AVAILABLE_DOCS+=("${FEATURE_DIR}/contracts/")
fi

DOCS_JSON=$(printf '%s,' "${AVAILABLE_DOCS[@]}")
DOCS_JSON="[${DOCS_JSON%,}]"

cat <<EOF
{
  "feature_dir": "${FEATURE_DIR}",
  "available_docs": ${DOCS_JSON}
}
EOF
