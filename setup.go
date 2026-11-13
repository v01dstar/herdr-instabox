package main

import (
	"os"
	"path/filepath"
)

func writeStarterConfig() error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(configPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err // Never overwrite, including a file created during the prompt.
	}
	_, writeErr := f.WriteString(starterConfig)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
