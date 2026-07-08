#!/usr/bin/env bash
set -euo pipefail

workspace_root="${1:?workspace root required}"
cli_bin="${2:?cli binary required}"
godot_version="${3:?godot version required}"
target_tuple="${4:?target tuple required}"
mode="${5:?integration mode required (verify|compile)}"
expected_release="${6:?expected release template required}"
expected_debug="${7:?expected debug template required}"
fixture_dir="${8:?fixture dir required}"

project_dir="${workspace_root}/integration-project"
rm -rf "${project_dir}"
mkdir -p "${project_dir}"
cp -R "${fixture_dir}/." "${project_dir}/"

pushd "${project_dir}" >/dev/null
case "${mode}" in
	verify|compile)
		;;
	*)
		echo "invalid integration mode: ${mode} (expected verify or compile)" >&2
		exit 2
		;;
esac

gst_args=(
	create
	--force
	--godot-version "${godot_version}"
	--platform "${target_tuple}"
)

if [[ "${mode}" == "verify" ]]; then
	gst_args+=(--verify-only)
else
	gst_args+=(--force-rebuild)
fi

"${cli_bin}" "${gst_args[@]}"

if [[ "${mode}" == "verify" ]]; then
	test -d ".gst/runtime/python"
	test -d ".gst/runtime/zig"
	test -d ".gst/runtime/scons"
	test -d ".gst/runtime/godot_source"

	test ! -f ".gst/manifest.json"
	test ! -f ".gst/encryption.key"
	test ! -f ".gst/templates/${expected_release}"
	test ! -f ".gst/templates/${expected_debug}"

	popd >/dev/null
	exit 0
fi

test -f ".gst/templates/${expected_release}"
test -f ".gst/templates/${expected_debug}"
test -f ".gst/manifest.json"
test -f ".gst/encryption.key"

popd >/dev/null