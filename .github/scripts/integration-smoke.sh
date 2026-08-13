#!/usr/bin/env bash
set -euo pipefail

workspace_root="${1:?workspace root required}"
cli_bin="${2:?cli binary required}"
godot_version="${3:?godot version required}"
target_tuple="${4:?target tuple required}"
fixture_dir="${5:?fixture dir required}"
sanitize_host_env="${6:-false}"

project_dir="${workspace_root}/integration-project"
rm -rf "${project_dir}"
mkdir -p "${project_dir}"
cp -R "${fixture_dir}/." "${project_dir}/"

pushd "${project_dir}" >/dev/null

gst_args=(
	create
	--force
	--force-rebuild
	--godot-version "${godot_version}"
	--platform "${target_tuple}"
)

if [[ "${sanitize_host_env}" == "true" ]]; then
	unset MSYSTEM || true
	unset MINGW_PREFIX || true
	unset CC || true
	unset CXX || true
	unset AR || true
	unset CFLAGS || true
	unset CXXFLAGS || true
fi

log_file="${project_dir}/gst-integration.log"

set +e
"${cli_bin}" "${gst_args[@]}" > >(tee "${log_file}") 2>&1 &
gst_pid=$!
set -e

scons_invocation_regex='scons: Building targets \.\.\.'
compile_progress_regex='Compiling .+ \.\.\.'
fatal_runtime_regex='scons: \*\*\*|Error: SCons build failed'
required_compile_lines=25
scons_invoked="false"
compile_deadline=$((SECONDS + 300))

while kill -0 "${gst_pid}" 2>/dev/null; do
	if [[ "${scons_invoked}" != "true" ]] && grep -Eq "${scons_invocation_regex}" "${log_file}"; then
		scons_invoked="true"
	fi

	if grep -Eq "${fatal_runtime_regex}" "${log_file}"; then
		echo "compile startup smoke detected a build error" >&2
		kill -TERM "${gst_pid}" 2>/dev/null || true
		wait "${gst_pid}" || true
		exit 8
	fi

	compile_line_count="$(grep -Ec "${compile_progress_regex}" "${log_file}" || true)"
	if (( compile_line_count >= required_compile_lines )); then
		break
	fi

	if (( SECONDS >= compile_deadline )); then
		if (( compile_line_count == 0 )); then
			if [[ "${scons_invoked}" == "true" ]]; then
				echo "timed out waiting for actual SCons compile output" >&2
			else
				echo "timed out waiting for SCons to start" >&2
			fi
		else
			echo "timed out waiting for SCons compile progress (${required_compile_lines} lines)" >&2
		fi
		kill -TERM "${gst_pid}" 2>/dev/null || true
		wait "${gst_pid}" || true
		exit 8
	fi

	sleep 1
done

compile_line_count="$(grep -Ec "${compile_progress_regex}" "${log_file}" || true)"
if (( compile_line_count < required_compile_lines )); then
	if grep -Eq "${fatal_runtime_regex}" "${log_file}"; then
		echo "SCons compile started but failed before required progress (${required_compile_lines} lines)" >&2
	else
		echo "SCons compile startup did not reach required progress (${compile_line_count}/${required_compile_lines})" >&2
	fi
	kill -TERM "${gst_pid}" 2>/dev/null || true
	wait "${gst_pid}" || true
	exit 8
fi

kill -TERM "${gst_pid}" 2>/dev/null || true
wait "${gst_pid}" || true

test -d ".gst/runtime/python"
if [[ "${target_tuple}" == "windows/amd64" ]]; then
	test -d ".gst/runtime/mingw"
else
	test -d ".gst/runtime/zig"
fi
test -d ".gst/runtime/scons"
test -d ".gst/runtime/godot_source"

popd >/dev/null
exit 0
