package autostart

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInstallWritesPlist(t *testing.T) {
	home := t.TempDir()
	loadLaunchd = func(string) error { return nil }
	t.Cleanup(func() { loadLaunchd = func(plist string) error { return exec.Command("launchctl", "load", plist).Run() } })

	plist, err := Install("/usr/local/bin/tokenledger", home)
	if err != nil {
		t.Fatal(err)
	}
	if plist != PlistPath(home) {
		t.Fatalf("plist = %s", plist)
	}
	data, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"com.blastersystems.tokenledger", "/usr/local/bin/tokenledger", "<string>up</string>", "RunAtLoad"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("plist missing %q", want)
		}
	}
	// Uninstall removes it.
	removed, err := Uninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if removed != plist {
		t.Fatalf("removed = %s", removed)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatal("plist still present")
	}
}

func TestInstallRejectsRelativePath(t *testing.T) {
	home := t.TempDir()
	if _, err := Install("tokenledger", home); err == nil {
		t.Fatal("relative path accepted")
	}
}

func TestUninstallMissingIsNoop(t *testing.T) {
	home := t.TempDir()
	removed, err := Uninstall(home)
	if err != nil || removed != "" {
		t.Fatalf("removed = %s err = %v", removed, err)
	}
}
