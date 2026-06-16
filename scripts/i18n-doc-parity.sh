#!/usr/bin/env bash
#
# i18n-doc-parity.sh
#
# For every *.md file at the repo root, if a *.zh.md translation exists,
# verify that the H2/H3 heading STRUCTURE is 1:1 with the source.
#
# What "structure" means here, per SPEC.md §3.2:
#   "H2/H3 headings must be 1:1 between source and translation. A
#    `## Quick Start` in English must be `## Quick start` or
#    `## 快速开始` in Chinese — but it must exist and be at the
#    same nesting level. This is enforceable by the verification
#    script (`grep -c "^## "`)."
#
# Concretely we check:
#   - same H2 count
#   - same H3 count
#   - same ordered sequence of (level, depth-relative position) — i.e.
#     a `## A / ## B / ### A.1 / ## C` structure in English must appear
#     in Chinese as `## X / ## Y / ### X.1 / ## Z` (same nesting order).
#
# We deliberately do NOT compare heading text — the translation can
# legitimately be "Quick start" → "快速开始" — only the structural
# presence and nesting level.
#
# Exit code: 0 if all matched, 1 on any mismatch.
#
# Usage:  bash scripts/i18n-doc-parity.sh

set -euo pipefail

# Resolve repo root from this script's location so it works from any cwd.
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &>/dev/null && pwd)"

cd "${REPO_ROOT}"

# Match lines that start with `## ` or `### ` followed by text.
H2_PATTERN='^## .*$'
H3_PATTERN='^### .*$'

# Extract the ordered sequence of heading levels (2 or 3) for one file.
# Output: one integer per line (the leading hash count).
heading_levels() {
  grep -E '^(##|###) ' "$1" | awk '{ print length($1) }'
}

mismatches=0
checked=0

for src in *.md; do
  # Skip non-source files: translations themselves and internal-only docs.
  case "${src}" in
    *.zh.md|*.zh-*.md) continue ;;  # already a translation, not a source
    CLAUDE.md|TODOS.md) continue ;; # internal, not user-facing docs
  esac

  base="${src%.md}"
  zh="${base}.zh.md"

  if [[ ! -f "${zh}" ]]; then
    # No translation exists for this doc — not a parity error.
    continue
  fi

  checked=$((checked + 1))

  src_h2_count=$(grep -cE "${H2_PATTERN}" "${src}" || true)
  zh_h2_count=$(grep -cE "${H2_PATTERN}" "${zh}"  || true)
  src_h3_count=$(grep -cE "${H3_PATTERN}" "${src}" || true)
  zh_h3_count=$(grep -cE "${H3_PATTERN}" "${zh}"  || true)

  src_levels=$(heading_levels "${src}" | tr '\n' ' ' | sed 's/ $//')
  zh_levels=$(heading_levels "${zh}"  | tr '\n' ' ' | sed 's/ $//')

  failed=0
  if [[ "${src_h2_count}" -ne "${zh_h2_count}" ]]; then
    echo "MISMATCH: ${src} vs ${zh}" >&2
    echo "  H2 count: src=${src_h2_count} zh=${zh_h2_count}" >&2
    failed=1
  fi
  if [[ "${src_h3_count}" -ne "${zh_h3_count}" ]]; then
    if [[ "${failed}" -eq 0 ]]; then
      echo "MISMATCH: ${src} vs ${zh}" >&2
    fi
    echo "  H3 count: src=${src_h3_count} zh=${zh_h3_count}" >&2
    failed=1
  fi
  if [[ "${src_levels}" != "${zh_levels}" ]]; then
    if [[ "${failed}" -eq 0 ]]; then
      echo "MISMATCH: ${src} vs ${zh}" >&2
    fi
    echo "  Heading nesting sequence differs." >&2
    echo "  src: ${src_levels}" >&2
    echo "  zh:  ${zh_levels}" >&2
    failed=1
  fi

  if [[ "${failed}" -eq 1 ]]; then
    mismatches=$((mismatches + 1))
    echo >&2
  fi
done

if [[ "${mismatches}" -gt 0 ]]; then
  echo "i18n-doc-parity: ${mismatches} file(s) out of ${checked} checked have heading drift." >&2
  exit 1
fi

if [[ "${checked}" -eq 0 ]]; then
  echo "i18n-doc-parity: no translated docs found at repo root (nothing to check)."
  exit 0
fi

echo "i18n-doc-parity: all matched (${checked} file(s))."
exit 0
