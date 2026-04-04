package install

import "testing"

func TestAssetNames(t *testing.T) {
	version := "v1.2.3"
	if got := PackageAssetName(version); got != "tmux-ghostty_v1.2.3_darwin_universal.pkg" {
		t.Fatalf("PackageAssetName() = %q", got)
	}
	if got := ArchiveAssetName(version); got != "tmux-ghostty_v1.2.3_darwin_universal.tar.gz" {
		t.Fatalf("ArchiveAssetName() = %q", got)
	}
}

func TestDetectInstallationMethod(t *testing.T) {
	testCases := []struct {
		path string
		want InstallationMethod
	}{
		{path: "/opt/homebrew/Cellar/tmux-ghostty/1.2.3/bin/tmux-ghostty", want: InstallationMethodHomebrew},
		{path: "/usr/local/Cellar/tmux-ghostty/1.2.3/bin/tmux-ghostty-broker", want: InstallationMethodHomebrew},
		{path: "/usr/local/bin/tmux-ghostty", want: InstallationMethodDirect},
		{path: "", want: InstallationMethodUnknown},
	}

	for _, testCase := range testCases {
		if got := DetectInstallationMethod(testCase.path); got != testCase.want {
			t.Fatalf("DetectInstallationMethod(%q) = %q, want %q", testCase.path, got, testCase.want)
		}
	}
}

func TestHomebrewFormulaNameDefault(t *testing.T) {
	if got := HomebrewFormulaName(); got != DefaultHomebrewFormulaName {
		t.Fatalf("HomebrewFormulaName() = %q", got)
	}
}
