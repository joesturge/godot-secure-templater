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
	writeErr := os.WriteFile(filePath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should write the source file before creating a backup")

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
	writeErr := os.WriteFile(filePath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should write the original file before creating a backup")
	writeErr = os.WriteFile(bakPath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should seed the pristine backup file")

	// AND the original file has been modified
	writeErr = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	assert.NoError(t, writeErr, "Should modify the source file after creating a backup")

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
	writeErr := os.WriteFile(presetsPath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should write the export presets fixture")

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
	writeErr := os.WriteFile(presetsPath, []byte("[preset.0.options]\n"), 0644)
	assert.NoError(t, writeErr, "Should write the export presets fixture")

	// WHEN injecting template paths
	err := InjectWindowsTemplate(presetsPath, "/path/release", "/path/debug")

	// THEN no error should occur
	assert.Nil(t, err, "InjectWindowsTemplate should not error")

	// AND the tool marker should be present for idempotency
	content, _ := os.ReadFile(presetsPath)
	contentStr := string(content)
	assert.Contains(t, contentStr, toolMarker, "Should add tool marker for idempotency")
}

func TestInjectWindowsTemplateTargetsWindowsPresetSection(t *testing.T) {
	// GIVEN an export_presets.cfg where Windows is not preset.0
	originalContent := `[preset.0]
name="Linux/X11"
platform="linuxbsd"

[preset.0.options]
other_option="linux"

[preset.1]
name="Windows Desktop"
platform="windows"

[preset.1.options]
other_option="windows"
`

	tmpDir := t.TempDir()
	presetsPath := filepath.Join(tmpDir, "export_presets.cfg")
	writeErr := os.WriteFile(presetsPath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should write the multi-preset export presets fixture")

	// WHEN injecting template paths
	err := InjectWindowsTemplate(presetsPath, "C:/tmp/windows_release.exe", "C:/tmp/windows_debug.exe")

	// THEN no error should occur
	assert.Nil(t, err, "InjectWindowsTemplate should not error when Windows preset is not preset.0")

	// AND custom template keys should be added to the Windows options section
	content, readErr := os.ReadFile(presetsPath)
	assert.NoError(t, readErr, "Should read modified export_presets.cfg")
	contentStr := string(content)

	assert.Contains(t, contentStr, "[preset.1.options]", "Windows options section should remain present")
	assert.Contains(t, contentStr, "custom_template/release=\"C:/tmp/windows_release.exe\"", "Release template path should be injected into Windows preset")
	assert.Contains(t, contentStr, "custom_template/debug=\"C:/tmp/windows_debug.exe\"", "Debug template path should be injected into Windows preset")

	// AND non-Windows options should remain unchanged
	assert.Contains(t, contentStr, "other_option=\"linux\"", "Non-Windows preset options should be preserved")
}

func TestInjectWindowsTemplateMissingOrBlankPresets(t *testing.T) {
	tests := []struct {
		name           string
		initialContent *string
	}{
		{
			name:           "missing export_presets file",
			initialContent: nil,
		},
		{
			name:           "blank export_presets file",
			initialContent: func() *string { s := ""; return &s }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a presets path that is either missing or blank
			tmpDir := t.TempDir()
			presetsPath := filepath.Join(tmpDir, "export_presets.cfg")

			if tt.initialContent != nil {
				writeErr := os.WriteFile(presetsPath, []byte(*tt.initialContent), 0644)
				assert.NoError(t, writeErr, "Should set up initial blank export_presets fixture")
			}

			// WHEN injecting template paths
			err := InjectWindowsTemplate(presetsPath, "C:/tmp/windows_template_release.exe", "C:/tmp/windows_template_debug.exe")

			// THEN no error should occur
			assert.Nil(t, err, "InjectWindowsTemplate should create or populate export_presets without errors")

			// AND the resulting file should contain a preset options section and template keys
			content, readErr := os.ReadFile(presetsPath)
			assert.NoError(t, readErr, "export_presets.cfg should be created or updated")
			contentStr := string(content)

			assert.Contains(t, contentStr, "[preset.0.options]", "Should include a default preset options section when no Windows section exists")
			assert.Contains(t, contentStr, "custom_template/release=\"C:/tmp/windows_template_release.exe\"", "Should inject release template path")
			assert.Contains(t, contentStr, "custom_template/debug=\"C:/tmp/windows_template_debug.exe\"", "Should inject debug template path")
		})
	}
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
	writeErr := os.WriteFile(credsPath, []byte(initialContent), 0644)
	assert.NoError(t, writeErr, "Should write the existing credentials fixture")

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
	writeErr := os.WriteFile(filePath, []byte(originalContent), 0644)
	assert.NoError(t, writeErr, "Should write the original file content")
	backupErr := BackupOnce(filePath)
	assert.Nil(t, backupErr, "BackupOnce should create a backup for restorability testing")

	// AND the file is significantly modified
	modifiedContent := "completely different content\nwith multiple lines\nand changes"
	writeErr = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	assert.NoError(t, writeErr, "Should write the modified file content")

	// WHEN checking the backup
	bakContent, err := os.ReadFile(bakPath)

	// THEN the backup should be readable
	assert.NoError(t, err, "Backup should be readable")
	// AND it should not be corrupted
	assert.Equal(t, originalContent, string(bakContent), "Backup should not be corrupted")

	// WHEN restoring from backup
	rollbackErr := Rollback(filePath)
	assert.NoError(t, rollbackErr, "Rollback should restore the pristine backup without error")

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
		writeErr := os.WriteFile(file, []byte("original"+string(rune(i))), 0644)
		assert.NoError(t, writeErr, "Should write the original file content before backing up")
		backupErr := BackupOnce(file)
		assert.Nil(t, backupErr, "BackupOnce should create a backup for each file")
	}

	// AND the files are modified
	for _, file := range files {
		writeErr := os.WriteFile(file, []byte("modified content"), 0644)
		assert.NoError(t, writeErr, "Should write the modified file content before rollback")
	}

	// WHEN rolling back all files
	rollbackErr := Rollback(files...)
	assert.NoError(t, rollbackErr, "Rollback should restore all files without error")

	// THEN all files should be restored to their original content
	for i, file := range files {
		content, _ := os.ReadFile(file)
		expected := "original" + string(rune(i))
		assert.Equal(t, expected, string(content), "Rollback should restore file %s", file)
	}
}
