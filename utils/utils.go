package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// AppDataDir returns an operating system specific directory to be used for
// storing application data for an application.  See Dir for more details.  This
// should only be used for directories where it is acceptable that the
// application name be visible in the path.
func AppDataDir(appName string, roaming bool) string {
	// The caller really shouldn't prepend the appName with a period, but
	// if they do, handle it gracefully by stripping it.
	if appName[0] == '.' {
		appName = appName[1:]
	}
	appNameUpper := string([]rune(appName)[0]) + appName[1:]
	appNameLower := appName

	// Get the OS specific home directory via the Go standard lib.
	var homeDir string
	home := os.Getenv("HOME")
	if home != "" {
		homeDir = home
	}

	// Fall back to standard dir if still not found
	if homeDir == "" {
		homeDir = "."
	}

	switch runtime.GOOS {
	// Microsoft Windows
	case "windows":
		// Allow override
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" || roaming {
			// Fall back to standard appdata
			dir = os.Getenv("APPDATA")
		}

		if dir != "" {
			return filepath.Join(dir, appNameUpper)
		}

		// Fallback
		return filepath.Join(homeDir, "AppData", "Local", appNameUpper)

	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", appNameUpper)

	// Unix-like
	default:
		return filepath.Join(homeDir, "."+appNameLower)
	}
}

// GenerateRandomString generates a random string of the specified length.
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random string: %w", err)
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func CleanAndExpandPath(path string) string {
	// Nothing to do when no path is given.
	if path == "" {
		return path
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows cmd.exe-style
	// %VARIABLE%, but the variables can still be expanded via POSIX-style
	// $VARIABLE.
	path = os.ExpandEnv(path)

	if !strings.HasPrefix(path, "~") {
		return filepath.Clean(path)
	}

	// Expand initial ~ to the current user's home directory, or ~otheruser
	// to otheruser's home directory.  On Windows, both forward and backward
	// slashes can be used.
	path = path[1:]

	var pathSeparators string
	if runtime.GOOS == "windows" {
		pathSeparators = string(os.PathSeparator) + "/"
	} else {
		pathSeparators = string(os.PathSeparator)
	}

	userName := ""
	if i := strings.IndexAny(path, pathSeparators); i != -1 {
		userName = path[:i]
		path = path[i:]
	}

	homeDir := ""
	var u *user.User
	var err error
	if userName == "" {
		u, err = user.Current()
	} else {
		u, err = user.Lookup(userName)
	}
	if err == nil {
		homeDir = u.HomeDir
	}
	// Fallback to CWD if user lookup fails or user has no home directory.
	if homeDir == "" {
		homeDir = "."
	}

	return filepath.Join(homeDir, path)
}
