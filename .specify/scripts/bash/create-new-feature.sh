#!/usr/bin/env bash
set -euo pipefail

NUMBER=""
SHORT_NAME=""
DESCRIPTION=""
JSON_MODE=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --number)
      NUMBER="$2"
      shift 2
      ;;
    --short-name)
      SHORT_NAME="$2"
      shift 2
      ;;
    --json)
      JSON_MODE=true
      shift
      ;;
    *)
      DESCRIPTION="$1"
      shift
      ;;
  esac
done

if [[ -z "$NUMBER" || -z "$SHORT_NAME" ]]; then
  echo "Error: --number and --short-name are required" >&2
  exit 1
fi

BRANCH_NAME="${NUMBER}-${SHORT_NAME}"
SPEC_DIR="specs/${BRANCH_NAME}"
SPEC_FILE="${SPEC_DIR}/spec.md"
FEATURE_DIR="${SPEC_DIR}"

mkdir -p "${SPEC_DIR}/checklists" "${SPEC_DIR}/contracts" "${SPEC_DIR}/research"

cp .specify/templates/spec-template.md "${SPEC_FILE}"

git checkout -b "${BRANCH_NAME}" 2>/dev/null || git checkout "${BRANCH_NAME}"

if $JSON_MODE; then
  cat <<EOF
{
  "branch_name": "${BRANCH_NAME}",
  "spec_file": "${SPEC_FILE}",
  "feature_dir": "${FEATURE_DIR}",
  "description": "${DESCRIPTION}"
}
EOF
else
  echo "Branch: ${BRANCH_NAME}"
  echo "Spec file: ${SPEC_FILE}"
  echo "Feature dir: ${FEATURE_DIR}"
fi
