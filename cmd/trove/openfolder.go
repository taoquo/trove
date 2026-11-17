package main

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// openFolder shows a directory in the system's file manager.
//
// Only a directory that exists is accepted. The path comes from the app's own data
// directory, never from a bundle, but the check is what keeps this function from
// becoming a general "ask the system to open a path" helper: the browser opener in
// openurl.go refuses anything that is not web precisely for that reason.
func openFolder(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("refusing to open a path that is not a folder")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "windows":
		return exec.Command("explorer", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
