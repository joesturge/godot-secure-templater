package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
)

const toolMarker = "# [gst managed]"

// BackupOnce creates a .bak file only if it doesn't exist.
func BackupOnce(filePath string) *internal.Error {
	bakPath := filePath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		// Backup already exists; don't overwrite.
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to read file for backup: %s", filePath),
			Details: err.Error(),
		}
	}

	if err := os.WriteFile(bakPath, data, 0644); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to write backup: %s", bakPath),
			Details: err.Error(),
		}
	}

	return nil
}

// InjectWindowsTemplate injects custom template paths into export_presets.cfg.
// Targets the Windows preset and sets custom_template/release and custom_template/debug.
func InjectWindowsTemplate(presetsPath, releasePath, debugPath string) *internal.Error {
	if err := BackupOnce(presetsPath); err != nil {
		return err
	}

	lines, err := os.ReadFile(presetsPath)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to read export_presets.cfg.",
			Details: err.Error(),
		}
	}

	// Parse lines, find the Windows preset section, and inject the template paths.
	newLines := injectTemplateKeys(string(lines), "preset.0.options", []struct {
		key   string
		value string
	}{
		{"custom_template/release", "\"" + releasePath + "\""},
		{"custom_template/debug", "\"" + debugPath + "\""},
	})

	if err := atomicWrite(presetsPath, newLines); err != nil {
		return err
	}

	return nil
}

// InjectEncryptionKey injects the encryption key into .godot/export_credentials.cfg (Godot 4.3+).
func InjectEncryptionKey(credsPath, key string) *internal.Error {
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(credsPath), 0755); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to create credentials directory.",
			Details: err.Error(),
		}
	}

	if err := BackupOnce(credsPath); err != nil {
		// Backup-once is optional for the first-time-create case.
		if _, statErr := os.Stat(credsPath); statErr != nil && os.IsNotExist(statErr) {
			// File doesn't exist; create it fresh.
		} else if statErr != nil {
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: "Failed to check credentials file.",
				Details: statErr.Error(),
			}
		}
	}

	// Read existing content or start empty.
	lines := ""
	if data, err := os.ReadFile(credsPath); err == nil {
		lines = string(data)
	}

	// Inject the encryption key.
	newLines := injectEncryptionSection(lines, key)

	if err := atomicWrite(credsPath, newLines); err != nil {
		return err
	}

	return nil
}

// injectTemplateKeys injects key-value pairs into a [section].
func injectTemplateKeys(content, section string, keys []struct {
	key   string
	value string
}) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var result []string
	inSection := false
	foundKeys := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for section header.
		if strings.HasPrefix(strings.TrimSpace(line), "["+section+"]") {
			inSection = true
			result = append(result, line)
			continue
		}

		// If we were in the target section and hit a new section, insert missing keys.
		if inSection && strings.HasPrefix(strings.TrimSpace(line), "[") {
			for _, kv := range keys {
				if !foundKeys[kv.key] {
					result = append(result, fmt.Sprintf("%s=%s %s", kv.key, kv.value, toolMarker))
				}
			}
			inSection = false
		}

		// Replace keys in the target section.
		if inSection {
			replaced := false
			for _, kv := range keys {
				if strings.HasPrefix(strings.TrimSpace(line), kv.key+"=") {
					result = append(result, fmt.Sprintf("%s=%s %s", kv.key, kv.value, toolMarker))
					foundKeys[kv.key] = true
					replaced = true
					break
				}
			}
			if !replaced {
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	// Add any remaining uninserted keys at EOF.
	if inSection {
		for _, kv := range keys {
			if !foundKeys[kv.key] {
				result = append(result, fmt.Sprintf("%s=%s %s", kv.key, kv.value, toolMarker))
			}
		}
	}

	return strings.Join(result, "\n")
}

// injectEncryptionSection injects or updates the encryption key in export_credentials.cfg.
func injectEncryptionSection(content, key string) string {
	// Ensure [encryption] section exists; inject script_encryption_key.
	scanner := bufio.NewScanner(strings.NewReader(content))
	var result []string
	inEncryption := false
	foundKey := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(strings.TrimSpace(line), "[encryption]") {
			inEncryption = true
			result = append(result, line)
			continue
		}

		if inEncryption && strings.HasPrefix(strings.TrimSpace(line), "[") {
			if !foundKey {
				result = append(result, fmt.Sprintf("script_encryption_key=\"%s\" %s", key, toolMarker))
			}
			inEncryption = false
		}

		if inEncryption && strings.HasPrefix(strings.TrimSpace(line), "script_encryption_key=") {
			result = append(result, fmt.Sprintf("script_encryption_key=\"%s\" %s", key, toolMarker))
			foundKey = true
		} else {
			result = append(result, line)
		}
	}

	// If no [encryption] section exists, create it.
	if !inEncryption && !foundKey {
		if !strings.Contains(content, "[encryption]") {
			result = append(result, "[encryption]")
		}
		result = append(result, fmt.Sprintf("script_encryption_key=\"%s\" %s", key, toolMarker))
	}

	return strings.Join(result, "\n")
}

// atomicWrite writes content to a file atomically (temp + rename).
func atomicWrite(path, content string) *internal.Error {
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to write temp file: %s", tmpPath),
			Details: err.Error(),
		}
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to finalize write: %s", path),
			Details: err.Error(),
		}
	}

	return nil
}

// Rollback restores files from their .bak counterparts.
func Rollback(paths ...string) error {
	for _, path := range paths {
		bakPath := path + ".bak"
		if data, err := os.ReadFile(bakPath); err == nil {
			os.WriteFile(path, data, 0644)
		}
	}
	return nil
}
