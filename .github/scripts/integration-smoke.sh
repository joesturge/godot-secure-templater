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
	--force-rebuild
	--godot-version "${godot_version}"
	--platform "${target_tuple}"
)

if [[ "${mode}" == "verify" ]]; then
	if [[ "${assert_zig_provenance}" == "true" ]]; then
		gst_args+=(--verbose)
	fi
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

if [[ "${mode}" == "verify" ]]; then
	set +e
	"${cli_bin}" "${gst_args[@]}" > >(tee "${log_file}") 2>&1 &
	gst_pid=$!
	set -e

	startup_signal_regex='scons: Building targets \.\.\.|Compiling .+ \.\.\.'
	compile_progress_regex='Compiling .+ \.\.\.'
	fatal_runtime_regex='scons: \*\*\*|Error: SCons build failed|exit status [0-9]+'
	required_compile_lines=25
	compile_started="false"
	startup_observed="false"
	compile_deadline=$((SECONDS + 300))
	while kill -0 "${gst_pid}" 2>/dev/null; do
		if [[ "${compile_started}" != "true" ]]; then
			if grep -Eq "${startup_signal_regex}" "${log_file}"; then
				compile_started="true"
			fi
		else
			if grep -Eq "${fatal_runtime_regex}" "${log_file}"; then
				echo "compile startup smoke detected a build error" >&2
				kill -TERM "${gst_pid}" 2>/dev/null || true
				wait "${gst_pid}" || true
				exit 8
			fi

			compile_line_count="$(grep -Ec "${compile_progress_regex}" "${log_file}" || true)"
			if (( compile_line_count >= required_compile_lines )); then
				startup_observed="true"
				break
			fi
		fi

		if [[ "${compile_started}" != "true" ]] && (( SECONDS >= compile_deadline )); then
			echo "timed out waiting for SCons compile start" >&2
			kill -TERM "${gst_pid}" 2>/dev/null || true
			wait "${gst_pid}" || true
			exit 8
		fi

		sleep 1
	done

	if [[ "${compile_started}" != "true" ]]; then
		wait "${gst_pid}" || true
		echo "SCons compile did not start" >&2
		exit 8
	fi

	if [[ "${startup_observed}" != "true" ]]; then
		echo "SCons compile startup did not reach required compile progress (${required_compile_lines} lines)" >&2
		kill -TERM "${gst_pid}" 2>/dev/null || true
		wait "${gst_pid}" || true
		exit 8
	fi

	kill -TERM "${gst_pid}" 2>/dev/null || true
	wait "${gst_pid}" || true
else
	"${cli_bin}" "${gst_args[@]}" 2>&1 | tee "${log_file}"
fi

if [[ "${mode}" == "verify" ]]; then
	test -d ".gst/runtime/python"
	test -d ".gst/runtime/zig"
	test -d ".gst/runtime/scons"
	test -d ".gst/runtime/godot_source"
	test -f ".gst/encryption.key"

	if [[ "${target_tuple}" == "windows/amd64" ]]; then
		test -d ".gst/runtime/zig-shims"
		test -d ".gst/runtime/zig-shims/bin"
	fi

	grep -Eq "${compile_progress_regex}" "${log_file}"

	if grep -Eq "${fatal_runtime_regex}" "${log_file}"; then
		echo "startup compile smoke detected an error during observation" >&2
		exit 8
	fi

	compile_line_count="$(grep -Ec "${compile_progress_regex}" "${log_file}" || true)"
	if (( compile_line_count < required_compile_lines )); then
		echo "startup compile smoke observed insufficient compile progress (${compile_line_count}/${required_compile_lines})" >&2
		exit 8
	fi

	if [[ "${assert_zig_provenance}" == "true" ]]; then
		if [[ "${target_tuple}" == "windows/amd64" ]]; then
			test -f ".gst/runtime/zig-shims/bin/clang.cmd"
			test -f ".gst/runtime/zig-shims/bin/clang++.cmd"
			test -f ".gst/runtime/zig-shims/bin/ar.cmd"
			grep -F 'zig cc %*' ".gst/runtime/zig-shims/bin/clang.cmd"
			grep -F 'zig c++ %*' ".gst/runtime/zig-shims/bin/clang++.cmd"
			grep -F 'zig ar %*' ".gst/runtime/zig-shims/bin/ar.cmd"
			grep -E 'Compiling .+\.llvm\.o' "${log_file}"
		fi
	fi

	test ! -f ".gst/manifest.json"
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