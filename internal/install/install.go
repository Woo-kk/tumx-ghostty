package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guyuanshun/tmux-ghostty/internal/buildinfo"
)

const (
	BinaryName                 = "tmux-ghostty"
	BrokerBinaryName           = "tmux-ghostty-broker"
	DefaultInstallDir          = "/usr/local/bin"
	ChecksumsAssetName         = "checksums.txt"
	DefaultHomebrewFormulaName = "tmux-ghostty"
)

type InstallationMethod string

const (
	InstallationMethodUnknown  InstallationMethod = "unknown"
	InstallationMethodDirect   InstallationMethod = "direct"
	InstallationMethodHomebrew InstallationMethod = "homebrew"
)

type Installation struct {
	Method         InstallationMethod `json:"method"`
	ExecutablePath string             `json:"executable_path"`
	ResolvedPath   string             `json:"resolved_path"`
	BinaryDir      string             `json:"binary_dir"`
}

func InstallDir() string {
	if dir := strings.TrimSpace(os.Getenv("TMUX_GHOSTTY_INSTALL_DIR")); dir != "" {
		return dir
	}
	return DefaultInstallDir
}

func MainBinaryPath() string {
	return filepath.Join(InstallDir(), BinaryName)
}

func BrokerBinaryPath() string {
	return filepath.Join(InstallDir(), BrokerBinaryName)
}

func ReleaseRepo() string {
	if repo := strings.TrimSpace(os.Getenv("TMUX_GHOSTTY_RELEASE_REPO")); repo != "" {
		return repo
	}
	return buildinfo.ReleaseRepo
}

func PackageID() string {
	if packageID := strings.TrimSpace(os.Getenv("TMUX_GHOSTTY_PACKAGE_ID")); packageID != "" {
		return packageID
	}
	return buildinfo.PackageID
}

func PackageAssetName(version string) string {
	return fmt.Sprintf("tmux-ghostty_%s_darwin_universal.pkg", version)
}

func ArchiveAssetName(version string) string {
	return fmt.Sprintf("tmux-ghostty_%s_darwin_universal.tar.gz", version)
}

func HomebrewFormulaName() string {
	if name := strings.TrimSpace(os.Getenv("TMUX_GHOSTTY_HOMEBREW_FORMULA")); name != "" {
		return name
	}
	return DefaultHomebrewFormulaName
}

func DetectInstallation() (Installation, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return Installation{}, err
	}
	resolvedPath := executablePath
	if realPath, err := filepath.EvalSymlinks(executablePath); err == nil && strings.TrimSpace(realPath) != "" {
		resolvedPath = realPath
	}
	return Installation{
		Method:         DetectInstallationMethod(resolvedPath),
		ExecutablePath: executablePath,
		ResolvedPath:   resolvedPath,
		BinaryDir:      filepath.Dir(resolvedPath),
	}, nil
}

func DetectInstallationMethod(path string) InstallationMethod {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if cleaned == "" {
		return InstallationMethodUnknown
	}
	if strings.Contains(cleaned, "/Cellar/") || strings.Contains(cleaned, "/Homebrew/Cellar/") {
		return InstallationMethodHomebrew
	}
	if strings.HasSuffix(cleaned, "/"+BinaryName) || strings.HasSuffix(cleaned, "/"+BrokerBinaryName) {
		return InstallationMethodDirect
	}
	return InstallationMethodUnknown
}
