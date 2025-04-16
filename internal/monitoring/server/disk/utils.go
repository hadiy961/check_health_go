package disk

import "strings"

// isExternalMount checks if a partition is from an external device
func isExternalMount(mountPoint, device string) bool {
	// Common paths for external or network mounts
	externalPaths := []string{"/media/", "/mnt/", "/run/media/"}
	for _, path := range externalPaths {
		if strings.HasPrefix(mountPoint, path) {
			return true
		}
	}

	// Network filesystems
	networkFS := []string{"nfs", "cifs", "smbfs", "ftpfs", "sshfs"}
	for _, fs := range networkFS {
		if strings.Contains(device, fs) {
			return true
		}
	}

	// Removable devices are often mounted in /dev/sd*
	if strings.Contains(device, "/dev/sd") && len(device) > 8 {
		// This is a heuristic and might need adjustment for specific environments
		return true
	}

	return false
}
