// Package autostart manages the per-user login item for the collector:
// a launchd agent on macOS and an HKCU Run entry on Windows. Never a
// system service; no admin rights.
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const label = "com.blastersystems.tokenledger"

// PlistPath returns the macOS launch agent path for the current user.
func PlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>up</string>
    <string>--no-open</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`

// Install writes (and loads on macOS) the per-user login item.
// exePath must be absolute. home is the user home directory.
func Install(exePath, home string) (string, error) {
	if !filepath.IsAbs(exePath) {
		return "", fmt.Errorf("exe path must be absolute: %s", exePath)
	}
	switch runtime.GOOS {
	case "darwin":
		return installDarwin(exePath, home)
	case "windows":
		return installWindows(exePath)
	case "linux":
		return "", fmt.Errorf("autostart is not supported on linux in v0.1 (run tokenledger up manually)")
	default:
		return "", fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}

// Uninstall removes the login item. It reports what it removed.
func Uninstall(home string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return uninstallDarwin(home)
	case "windows":
		return uninstallWindows()
	case "linux":
		return "", nil
	default:
		return "", nil
	}
}

// loadLaunchd loads a plist; var for test stubbing.
var loadLaunchd = func(plist string) error {
	return exec.Command("launchctl", "load", plist).Run()
}

func installDarwin(exePath, home string) (string, error) {
	plist := PlistPath(home)
	logPath := filepath.Join(home, ".tokenledger", "tokenledger.log")
	content := fmt.Sprintf(plistTemplate, label, exePath, logPath, logPath)
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(plist, []byte(content), 0o644); err != nil {
		return "", err
	}
	_ = exec.Command("launchctl", "unload", plist).Run()
	if err := loadLaunchd(plist); err != nil {
		return plist, fmt.Errorf("launchctl load: %v", err)
	}
	return plist, nil
}

func uninstallDarwin(home string) (string, error) {
	plist := PlistPath(home)
	if _, err := os.Stat(plist); os.IsNotExist(err) {
		return "", nil
	}
	_ = exec.Command("launchctl", "unload", plist).Run()
	if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
		return plist, err
	}
	return plist, nil
}

func installWindows(exePath string) (string, error) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	out, err := exec.Command("reg", "add", key, "/v", label, "/t", "REG_SZ", "/d", exePath+" up --no-open", "/f").CombinedOutput()
	if err != nil {
		return key, fmt.Errorf("reg add: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return key, nil
}

func uninstallWindows() (string, error) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	out, err := exec.Command("reg", "delete", key, "/v", label, "/f").CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(out)), "unable to find") {
		return key, fmt.Errorf("reg delete: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return key, nil
}

// RegistryValueName exposes the Windows Run-key value name for doctor.
func RegistryValueName() string { return label }
