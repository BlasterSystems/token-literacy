package autostart

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPureHelpers covers the OS-neutral helpers so the shared package keeps
// meaningful coverage on runners that cannot exercise launchd or the
// registry.
func TestPureHelpers(t *testing.T) {
	p := PlistPath(filepath.Join(string(filepath.Separator)+"home", "x"))
	if filepath.Dir(p) != filepath.Join(string(filepath.Separator)+"home", "x", "Library", "LaunchAgents") {
		t.Fatalf("plist dir = %q", filepath.Dir(p))
	}
	if !strings.HasSuffix(p, "tokenledger.plist") {
		t.Fatalf("plist path = %q", p)
	}
	if got := xmlEscape("a<&>\"'"); got != "a&lt;&amp;&gt;\"'" {
		t.Errorf("xmlEscape = %q", got)
	}
	if RegistryValueName() == "" {
		t.Error("registry value name empty")
	}
}

func TestInstallWritesPlist(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plist is darwin-only")
	}
	home := t.TempDir()
	loadLaunchd = func(string) error { return nil }
	t.Cleanup(func() {
		loadLaunchd = func(plist string) error { return exec.Command("launchctl", "load", plist).Run() }
	})

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
