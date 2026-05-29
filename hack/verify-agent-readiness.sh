#!/usr/bin/env bash
# Verify repository metadata used by coding agents.
set -o errexit
set -o nounset
set -o pipefail

max_agents_lines=60
agents_file="AGENTS.md"
skill_dir="skills/tekton-pipeline-discovery"
skill_file="${skill_dir}/SKILL.md"
expected_link_target="../../skills/tekton-pipeline-discovery"
skill_links=(
  ".agents/skills/tekton-pipeline-discovery"
  ".claude/skills/tekton-pipeline-discovery"
  ".codex/skills/tekton-pipeline-discovery"
)

if [[ ! -f "${agents_file}" ]]; then
  echo "${agents_file} is missing" >&2
  exit 1
fi

agents_lines=$(wc -l < "${agents_file}" | tr -d ' ')
if (( agents_lines > max_agents_lines )); then
  echo "${agents_file} has ${agents_lines} lines; maximum is ${max_agents_lines}" >&2
  exit 1
fi

if [[ ! -f "${skill_file}" ]]; then
  echo "${skill_file} is missing" >&2
  exit 1
fi

for skill_link in "${skill_links[@]}"; do
  if [[ ! -L "${skill_link}" ]]; then
    echo "${skill_link} must be a symlink" >&2
    exit 1
  fi

  actual_target=$(readlink "${skill_link}")
  if [[ "${actual_target}" != "${expected_link_target}" ]]; then
    echo "${skill_link} points to ${actual_target}; want ${expected_link_target}" >&2
    exit 1
  fi

  if [[ ! -d "${skill_link}" ]]; then
    echo "${skill_link} does not resolve to a directory" >&2
    exit 1
  fi
done

echo "agent readiness checks passed (${agents_file}: ${agents_lines}/${max_agents_lines} lines)"
