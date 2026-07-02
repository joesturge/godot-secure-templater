---
applyTo: "**/*_test.go"
---

# Test File Requirements

## BDD Comments (MANDATORY)
```go
func TestName(t *testing.T) {
	// GIVEN setup
	value := setup()
	// WHEN action
	result := Function(value)
	// THEN assertion
	assert.Equal(t, expected, result)
	// AND additional checks
	assert.True(t, condition)
}
```

## Assertions
- **ONLY** use `github.com/stretchr/testify/assert`
- Std errors: `assert.NoError(t, err)` | Custom `*Error`: `assert.Nil(t, err)`
- All messages must be non-empty: `assert.Equal(t, want, got, "message")`
- Never: `if err != nil { t.Errorf(...) }`

## Patterns
- Use `t.TempDir()` for filesystem tests (parallel-safe)
- Table-driven with subtests for multiple scenarios
- Each subtest needs GIVEN/WHEN/THEN comments
- Verify file permissions: `assert.Equal(t, os.FileMode(0600), info.Mode().Perm())`
- Error codes: `assert.Equal(t, ExitCode, err.Code)`

## Coverage
- Target: ~85% per package
- Check: `go test -cover ./...`
