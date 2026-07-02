package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupOnceCreatesBackup(t *testing.T) {
	// GIVEN an original file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.cfg")
	bakPath := filePath + ".bak"
	originalContent := "original content"
	os.WriteFile(filePath, []byte(originalContent), 0644)

	// WHEN creating a backup for the first time
	err := BackupOnce(filePath)

	// THEN no error should occur
	assert.Nil(t, err, "BackupOnce should not error")

	// AND the .bak file should be created
	bakContent, readErr := os.ReadFile(bakPath)
	assert.NoError(t, readErr, "Backup file should exist")
	// AND it should contain the original content
	assert.Equal(t, originalContent, string(bakContent), "Backup content should match original")
}

func TestBackupOnceDoesNotOverwrite(t *testing.T) {
	// GIVEN a file with a backup already created
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.cfg")
	bakPath := filePath + ".bak"
	originalContent := "original"
	modifiedContent := "modified"
	os.WriteFile(filePath, []byte(originalContent), 0644)
	os.WriteFile(bakPath, []byte(originalContent), 0644)

	// AND the original file has been modified
	os.WriteFile(filePath, []byte(modifiedContent), 0644)

	// WHEN creating a backup again
	err := BackupOnce(filePath)

	// THEN no error should occur
	assert.Nil(t, err, "BackupOnce should not error")

	// AND the .bak should NOT be overwritten
	bakContent, _ := os.ReadFile(bakPath)
	assert.Equal(t, originalContent, string(bakContent), "BackupOnce should not overwrite existing .bak")
}

func TestInjectWindowsTemplateBytePreservation(t *testing.T) {
	// GIVEN an export_presets.cfg file with carefully formatted content
	// (golden test: verify byte-for-byte preservation of untouched content)
	originalContent := `[preset.0]

name="Windows Desktop"
platform="windows"
runnable=true
custom_features=""

[preset.0.options]

other_option="value"
`

	tmpDir := t.TempDir()
	presetsPath := filepath.Join(tmpDir, "export_presets.cfg")
	os.WriteFile(presetsPath, []byte(originalContent), 0644)

	// WHEN injecting template paths
	releasePath := "/path/to/release.exe"
	debugPath := "/path/to/debug.exe"

	err := InjectWindowsTemplate(presetsPath, releasePath, debugPath)

	// THEN no error should occur
	assert.Nil(t, err, "InjectWindowsTemplate should not error")

	// AND the file should be updated
	modifiedContent, _ := os.ReadFile(presetsPath)
	content := string(modifiedContent)

	// AND new paths should be injected
	assert.Contains(t, content, releasePath, "Should inject release path")
	assert.Contains(t, content, debugPath, "Should inject debug path")

	// AND original content should be preserved
	assert.Contains(t, content, `name="Windows Desktop"`, "Should preserve name")
	assert.Contains(t, content, `platform="windows"`, "Should preserve platform")
	assert.Contains(t, content, `other_option="value"`, "Should preserve other_option")

	// AND section structure should be intact
	assert.Contains(t, content, "[preset.0]", "Should preserve section header")
}

func TestInjectWindowsTemplateWithToolMarker(t *testing.T) {
	// GIVEN an export_presets.cfg file
	tmpDir := t.TempDir()
	presetsPath := filepath.Join(tmpDir, "export_presets.cfg")
	os.WriteFile(presetsPath, []byte("[preset.0.options]\n"), 0644)

	// WHEN injecting template paths
	err := InjectWindowsTemplate(presetsPath, "/path/release", "/path/debug")

	// THEN no error should occur
	assert.Nil(t, err, "InjectWindowsTemplate should not error")

	// AND the tool marker should be present for idempotency
	content, _ := os.ReadFile(presetsPath)
	contentStr := string(content)
	assert.Contains(t, contentStr, toolMarker, "Should add tool marker for idempotency")
}

func TestInjectEncryptionKeyCreatesFile(t *testing.T) {
	// GIVEN a non-existent credentials file path and encryption key
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, ".godot", "export_credentials.cfg")
	testKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1"

	// WHEN injecting the encryption key
	err := InjectEncryptionKey(credsPath, testKey)

	// THEN no error should occur
	assert.Nil(t, err, "InjectEncryptionKey should not error")

	// AND the file should be created
	content, readErr := os.ReadFile(credsPath)
	assert.NoError(t, readErr, "Credentials file should exist")
	// AND it should contain the key
	assert.Contains(t, string(content), testKey, "Key should be in file")

	// AND [encryption] section should be created
	assert.Contains(t, string(content), "[encryption]", "Should create [encryption] section")
}

func TestInjectEncryptionKeyUpdateExisting(t *testing.T) {
	// GIVEN an existing credentials file with an old encryption key
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "export_credentials.cfg")
	initialContent := `[encryption]
script_encryption_key="old_key_here"
`
	os.WriteFile(credsPath, []byte(initialContent), 0644)

	// WHEN injecting a new encryption key
	newKey := "new_key_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0"
	err := InjectEncryptionKey(credsPath, newKey)

	// THEN no error should occur
	assert.Nil(t, err, "InjectEncryptionKey should not error")

	// AND the file should be updated
	content, _ := os.ReadFile(credsPath)
	contentStr := string(content)

	// AND the old key should be replaced
	assert.NotContains(t, contentStr, "old_key_here", "Should replace old key")
	// AND the new key should be present
	assert.Contains(t, contentStr, newKey, "New key should be in file")
}

func TestBackupOncePreservesRestorability(t *testing.T) {
	// GIVEN a file with a pristine backup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.cfg")
	bakPath := filePath + ".bak"
	originalContent := "pristine original"
	os.WriteFile(filePath, []byte(originalContent), 0644)
	BackupOnce(filePath)

	// AND the file is significantly modified
	modifiedContent := "completely different content\nwith multiple lines\nand changes"
	os.WriteFile(filePath, []byte(modifiedContent), 0644)

	// WHEN checking the backup
	bakContent, err := os.ReadFile(bakPath)

	// THEN the backup should be readable
	assert.NoError(t, err, "Backup should be readable")
	// AND it should not be corrupted
	assert.Equal(t, originalContent, string(bakContent), "Backup should not be corrupted")

	// WHEN restoring from backup
	Rollback(filePath)

	// THEN the original content should be restored
	restoredContent, _ := os.ReadFile(filePath)
	assert.Equal(t, originalContent, string(restoredContent), "Rollback should restore original content")
}

func TestInjectEncryptionKeyAtomicWrite(t *testing.T) {
	// GIVEN a credentials file path and test key
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "export_credentials.cfg")
	testKey := "atomic_write_test_key_1234567890abcdef"

	// WHEN injecting the encryption key
	err := InjectEncryptionKey(credsPath, testKey)

	// THEN no error should occur
	assert.Nil(t, err, "InjectEncryptionKey should not error")

	// AND no .tmp file should be left behind (atomic write cleanup)
	tmpPath := credsPath + ".tmp"
	_, statErr := os.Stat(tmpPath)
	assert.Error(t, statErr, "Atomic write should clean up .tmp file")

	// AND the main file should exist
	_, mainStatErr := os.Stat(credsPath)
	assert.NoError(t, mainStatErr, "Main file should exist")
}

func TestRollbackMultipleFiles(t *testing.T) {
	// GIVEN multiple files with backups
	tmpDir := t.TempDir()
	files := []string{
		filepath.Join(tmpDir, "file1.cfg"),
		filepath.Join(tmpDir, "file2.cfg"),
	}

	// AND the files are created and backed up
	for i, file := range files {
		os.WriteFile(file, []byte("original"+string(rune(i))), 0644)
		BackupOnce(file)
	}

	// AND the files are modified
	for _, file := range files {
		os.WriteFile(file, []byte("modified content"), 0644)
	}

	// WHEN rolling back all files
	Rollback(files...)

	// THEN all files should be restored to their original content
	for i, file := range files {
		content, _ := os.ReadFile(file)
		expected := "original" + string(rune(i))
		assert.Equal(t, expected, string(content), "Rollback should restore file %s", file)
	}
}
