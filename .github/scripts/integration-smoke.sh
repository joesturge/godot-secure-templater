#!/usr/bin/env bash
set -euo pipefail

workspace_root="${1:?workspace root required}"
cli_bin="${2:?cli binary required}"
godot_version="${3:?godot version required}"
target_tuple="${4:?target tuple required}"
expected_release="${5:?expected release template required}"
expected_debug="${6:?expected debug template required}"
fixture_dir="${7:?fixture dir required}"

project_dir="${workspace_root}/integration-project"
rm -rf "${project_dir}"
mkdir -p "${project_dir}"
cp -R "${fixture_dir}/." "${project_dir}/"

pushd "${project_dir}" >/dev/null
"${cli_bin}" create --force --force-rebuild --godot-version "${godot_version}" --platform "${target_tuple}"

test -f ".gst/templates/${expected_release}"
test -f ".gst/templates/${expected_debug}"
test -f ".gst/manifest.json"
test -f ".gst/encryption.key"

popd >/dev/null