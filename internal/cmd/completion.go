package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

func init() {
	completionCmd.Flags().BoolP("install", "i", false, "Automatically install completion script")
	RootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:                   "completion [bash|zsh|fish|powershell]",
	Short:                 "Generate shell completion script",
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:                  runCompletion,
}

// ShellHandler defines the behavior for a specific shell
type ShellHandler struct {
	Generate func(cmd *cobra.Command, w io.Writer) error
	Path     func(home string) string
	PostMsg  string
}

// handlers registry
var shellHandlers = map[string]ShellHandler{
	"bash":       bashHandler,
	"zsh":        zshHandler,
	"fish":       fishHandler,
	"powershell": powershellHandler,
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]
	install, _ := cmd.Flags().GetBool("install")

	handler := shellHandlers[shell]

	// 1. Output to stdout
	if !install {
		return handler.Generate(RootCmd, os.Stdout)
	}

	// 2. Install to file
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	targetPath := handler.Path(home)
	if err := installToFile(targetPath, handler.Generate); err != nil {
		return err
	}

	fmt.Printf("✓ Completion installed to: %s\n", targetPath)
	if handler.PostMsg != "" {
		fmt.Println(handler.PostMsg)
	}
	return nil
}

// --- Implementation Details ---

func installToFile(path string, genFunc func(*cobra.Command, io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	var buf bytes.Buffer
	if err := genFunc(RootCmd, &buf); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}
	return nil
}

// --- Shell Handlers Definitions ---

var bashHandler = ShellHandler{
	Generate: (*cobra.Command).GenBashCompletion,
	Path:     getBashPath,
	PostMsg:  "Please restart your shell to apply changes.",
}

var zshHandler = ShellHandler{
	Generate: (*cobra.Command).GenZshCompletion,
	Path:     func(h string) string { return filepath.Join(h, ".zfunc", "_snirect") },
	PostMsg:  getZshMsg(),
}

var fishHandler = ShellHandler{
	Generate: func(c *cobra.Command, w io.Writer) error { return c.GenFishCompletion(w, true) },
	Path:     func(h string) string { return filepath.Join(h, ".config", "fish", "completions", "snirect.fish") },
}

var powershellHandler = ShellHandler{
	Generate: (*cobra.Command).GenPowerShellCompletionWithDesc,
	Path:     getPowerShellPath,
	PostMsg:  "To enable, add the file path to your $PROFILE.",
}

// --- Path Helpers ---

func getBashPath(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, ".bash_completion.d", "snirect")
	}
	return filepath.Join(home, ".local", "share", "bash-completion", "completions", "snirect")
}

func getPowerShellPath(home string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Scripts", "snirect-completion.ps1")
	}
	return filepath.Join(home, ".config", "powershell", "snirect-completion.ps1")
}

func getZshMsg() string {
	if runtime.GOOS == "windows" {
		return "Please restart your shell."
	}
	return `Please restart your shell.
Note: Ensure ~/.zfunc is in your fpath in .zshrc:
      fpath+=~/.zfunc; autoload -U compinit; compinit`
}
