package detect

import "os"

// FileExists returns true if the path exists on disk.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
