//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName = "snirect"
	buildDir   = "dist"
	cmdPath    = "./cmd/snirect"
	rulesURL   = "https://github.com/SpaceTimee/Cealing-Host/raw/refs/heads/main/Cealing-Host.json"
)

var (
	// Default target
	Default = Build
)

// getVersion returns the version in format TAGVERSION[-N][-dirty]
// - TAGVERSION: latest git tag (e.g., v1.0.0)
// - N (optional): distance (commits ahead) from the tagged commit, only included if > 0
// - dirty (optional): append "-dirty" if there are unstaged changes
func getVersion() string {
	// Get the latest tag reachable from HEAD
	tag, err := sh.Output("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		// No tags found, fallback to commit hash
		hash, hashErr := sh.Output("git", "rev-parse", "--short", "HEAD")
		if hashErr != nil {
			return "0.0.0-dev"
		}
		// Check dirty status
		if isDirty() {
			return hash + "-dirty"
		}
		return hash
	}

	// Get distance from the tag to HEAD (number of commits)
	dist, err := sh.Output("git", "rev-list", "--count", tag+"..HEAD")
	if err != nil || dist == "" {
		dist = "0"
	}

	// Build version string: tag only if distance is 0, otherwise tag-distance
	version := tag
	if dist != "0" {
		version = fmt.Sprintf("%s-%s", tag, dist)
	}

	// Append dirty suffix if there are unstaged changes
	if isDirty() {
		version += "-dirty"
	}
	return version
}

// isDirty checks if there are any unstaged changes in the working directory
func isDirty() bool {
	// Check modified but unstaged files
	out, err := sh.Output("git", "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) != ""
}

// getLDFLAGS returns the linker flags
func getLDFLAGS() string {
	return fmt.Sprintf("-s -w -X 'snirect/internal/cmd.Version=%s'", getVersion())
}

// Generate runs code generation
func Generate() error {
	fmt.Println("Running code generation...")
	return sh.RunV("go", "generate", "./...")
}

// GenerateCompletions generates shell completion scripts
func GenerateCompletions() error {
	fmt.Println("Generating completions...")
	compDir := filepath.Join("internal", "cmd", "completions")
	os.RemoveAll(compDir)
	if err := os.MkdirAll(compDir, 0755); err != nil {
		return err
	}

	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		out, _ := sh.Output("go", "run", cmdPath, "completion", shell)
		if out != "" {
			err := os.WriteFile(filepath.Join(compDir, shell), []byte(out), 0644)
			if err != nil {
				fmt.Printf("Warning: failed to write %s completion: %v\n", shell, err)
			}
		}
	}
	return nil
}

// Build compiles the binary
func Build() error {
	mg.Deps(Generate)
	mg.Deps(GenerateCompletions)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	tags := os.Getenv("TAGS")
	return sh.RunV("go", "build", "-tags", tags, "-ldflags", getLDFLAGS(), "-o", filepath.Join(buildDir, binaryName), cmdPath)
}

// Release compiles the binary (currently identical to build)
func Release() error {
	return Build()
}

// Full builds with all features (includes QUIC)
func Full() error {
	os.Setenv("TAGS", "quic")
	return Release()
}

// Clean removes build artifacts
func Clean() {
	fmt.Println("Cleaning...")
	os.RemoveAll(buildDir)
	os.Remove("log.txt")
	os.RemoveAll(filepath.Join("internal", "cmd", "completions"))
}

// UpdateRules downloads and converts rules from upstream to shared library
func UpdateRules() error {
	fmt.Printf("Updating rules from %s...\n", rulesURL)
	rawPath := filepath.Join("internal", "config", "rules.raw.json")
	// Convert and write to shared library
	targetPath := filepath.Join("..", "snirect-shared", "rules", "fetched.toml")

	if err := sh.RunV("curl", "-sSL", rulesURL, "-o", rawPath); err != nil {
		return err
	}
	defer os.Remove(rawPath)

	// Call convert_rules from shared library
	return sh.RunV("go", "run", "github.com/xihale/snirect-shared/tools/convert_rules", rawPath, targetPath)
}

// CrossAll performs cross-platform builds
func CrossAll() error {
	mg.Deps(Clean)
	mg.Deps(Generate, GenerateCompletions)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	type target struct {
		os   string
		arch string
	}

	targets := []target{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(targets))

	for _, t := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			output := filepath.Join(buildDir, fmt.Sprintf("%s-%s-%s", binaryName, t.os, t.arch))
			if t.os == "windows" {
				output += ".exe"
			}

			fmt.Printf("Building for %s/%s...\n", t.os, t.arch)
			env := map[string]string{
				"GOOS":   t.os,
				"GOARCH": t.arch,
			}
			err := sh.RunWithV(env, "go", "build", "-ldflags", getLDFLAGS(), "-o", output, cmdPath)
			if err != nil {
				errs <- fmt.Errorf("failed build for %s/%s: %v", t.os, t.arch, err)
			}
		}(t)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// Checksum generates sha256 checksums for files in dist
func Checksum() error {
	files, err := filepath.Glob(filepath.Join(buildDir, "*"))
	if err != nil {
		return err
	}

	var checksums string
	for _, f := range files {
		if filepath.Base(f) == "checksums.txt" {
			continue
		}
		sum, err := sh.Output("sha256sum", f)
		if err != nil {
			// Try shasum -a 256 for macOS
			sum, err = sh.Output("shasum", "-a", "256", f)
			if err != nil {
				continue
			}
		}
		checksums += sum + "\n"
	}

	return os.WriteFile(filepath.Join(buildDir, "checksums.txt"), []byte(checksums), 0644)
}

// Install builds and runs the internal install logic
func Install() error {
	mg.Deps(Build)
	bin := filepath.Join(buildDir, binaryName)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return sh.RunV(bin, "install")
}

// Uninstall builds and runs the internal uninstall logic
func Uninstall() error {
	mg.Deps(Build)
	bin := filepath.Join(buildDir, binaryName)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return sh.RunV(bin, "uninstall")
}

// Upx compresses binaries in dist with UPX
func Upx() error {
	files, err := filepath.Glob(filepath.Join(buildDir, binaryName+"-*"))
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.Contains(f, "windows-arm64") {
			continue // UPX usually doesn't support windows/arm64 well or at all
		}
		sh.Run("upx", f)
	}
	return nil
}
