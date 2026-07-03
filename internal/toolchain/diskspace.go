package toolchain

import "github.com/joemi/godot-secure-templater/internal"

const minimumRequiredDiskBytes uint64 = 5 * 1024 * 1024 * 1024

var getAvailableDiskBytes = platformAvailableDiskBytes

// EnsureSufficientDiskSpace validates there is enough free space for toolchain/source provisioning.
func EnsureSufficientDiskSpace(path string, requiredBytes uint64) *internal.Error {
	availableBytes, err := getAvailableDiskBytes(path)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to determine available disk space.",
			Details: err.Error(),
		}
	}

	if availableBytes < requiredBytes {
		return internal.ErrInsufficientDisk(requiredBytes, availableBytes, path)
	}

	return nil
}