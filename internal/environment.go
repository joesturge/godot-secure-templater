package internal

import (
	"fmt"
	"os"
	"strings"
)

var toolchainEnvironmentKeys = map[string]struct{}{
	"AR": {}, "CC": {}, "CXX": {},
	"AS": {}, "LD": {}, "NM": {}, "OBJCOPY": {}, "OBJDUMP": {}, "RANLIB": {}, "RC": {}, "STRIP": {}, "WINDRES": {},
	"CFLAGS": {}, "CPPFLAGS": {}, "CXXFLAGS": {}, "LDFLAGS": {},
	"CPATH": {}, "C_INCLUDE_PATH": {}, "CPLUS_INCLUDE_PATH": {},
	"INCLUDE": {}, "LIB": {}, "LIBPATH": {},
	"PKG_CONFIG_PATH": {}, "PKG_CONFIG_LIBDIR": {},
	"MINGW_PREFIX": {}, "MSYSTEM": {},
	"EMSDK": {}, "EMSDK_ROOT": {}, "EM_CONFIG": {}, "EM_CACHE": {}, "EMCC_CFLAGS": {}, "EMCC_DEBUG": {},
	"PYTHONHOME": {}, "PYTHONPATH": {},
	"SCONSFLAGS": {}, "SCONS_LIB_DIR": {},
	"SDKROOT": {}, "MACOSX_DEPLOYMENT_TARGET": {},
	"PATH": {},
}

// SanitizedEnv preserves ordinary host settings while removing build-toolchain
// configuration unless gst supplies an explicit runtime override.
func SanitizedEnv(overrides map[string]string) []string {
	overridden := make(map[string]struct{}, len(overrides))
	for key := range overrides {
		overridden[strings.ToUpper(key)] = struct{}{}
	}

	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		_, isToolchainSetting := toolchainEnvironmentKeys[key]
		_, hasOverride := overridden[key]
		if !isToolchainSetting && !hasOverride {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}
