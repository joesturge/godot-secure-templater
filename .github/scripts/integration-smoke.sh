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
sanitize_host_env="${9:-false}"
assert_zig_provenance="${10:-false}"

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
	if [[ "${assert_zig_provenance}" == "true" ]]; then
		gst_args+=(--verbose)
	fi
else
	gst_args+=(--force-rebuild)
fi

if [[ "${sanitize_host_env}" == "true" ]]; then
	unset MINGW_PREFIX || true
	unset MSYSTEM || true
	unset CC || true
	unset CXX || true
	unset AR || true
	unset CFLAGS || true
	unset CXXFLAGS || true
fi

log_file="${project_dir}/gst-integration.log"
"${cli_bin}" "${gst_args[@]}" 2>&1 | tee "${log_file}"

if [[ "${mode}" == "verify" ]]; then
	test -d ".gst/runtime/python"
	test -d ".gst/runtime/zig"
	test -d ".gst/runtime/scons"
	test -d ".gst/runtime/godot_source"

	if [[ "${target_tuple}" == "windows/amd64" ]]; then
		test -d ".gst/runtime/zig-shims"
		test -d ".gst/runtime/zig-shims/bin"
	fi

	if [[ "${assert_zig_provenance}" == "true" ]]; then
		grep -F 'Verify-only compiler env: CC="zig-cc" CXX="zig-cxx" AR="zig-ar"' "${log_file}"
		grep -E 'Verify-only compiler env: .*MINGW_PREFIX=".+"' "${log_file}"
		grep -E 'Verify-only PATH head: .*zig-shims' "${log_file}"
		grep -E 'Verify-only SCons args: .*use_llvm=yes.*use_mingw=no' "${log_file}"
	fi

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