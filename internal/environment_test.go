package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizedEnvRejectsHostToolchainSetup(t *testing.T) {
	// GIVEN poisoned host toolchain variables and an ordinary locale variable
	poisoned := map[string]string{
		"CC":              "host-cc",
		"CFLAGS":          "-I/host/include",
		"LD":              "host-ld",
		"RANLIB":          "host-ranlib",
		"PYTHONHOME":      "/host/python",
		"EMSDK":           "/host/emsdk",
		"EM_CACHE":        "/host/em-cache",
		"SCONSFLAGS":      "--host-setting",
		"PKG_CONFIG_PATH": "/host/pkgconfig",
	}
	for key, value := range poisoned {
		t.Setenv(key, value)
	}
	t.Setenv("GST_TEST_LOCALE", "en_US.UTF-8")
	t.Setenv("Path", "/host/bin")

	// WHEN merging provisioned runtime overrides
	env := SanitizedEnv(map[string]string{
		"CC":   "runtime-cc",
		"PATH": "/runtime/bin",
	})
	values := map[string]string{}
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		values[strings.ToUpper(parts[0])] = parts[1]
	}

	// THEN runtime overrides win and unclaimed host toolchain variables are removed
	assert.Equal(t, "runtime-cc", values["CC"], "runtime compiler override should replace the host compiler")
	assert.Equal(t, "/runtime/bin", values["PATH"], "runtime PATH should replace host PATH regardless of key casing")
	for key := range poisoned {
		if key != "CC" {
			assert.NotContains(t, values, key, "host toolchain setting %s should not reach the build", key)
		}
	}
	assert.Equal(t, "en_US.UTF-8", values["GST_TEST_LOCALE"], "ordinary host environment should remain available")
}
