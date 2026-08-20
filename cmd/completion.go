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

var completionCmd = &cobra.Command{
	Use:                   "completion [bash|zsh|fish|powershell]",
	Short:                 "Generate shell completion script",
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:                  runCompletion,
}

type shellHandler struct {
	gen  func(*cobra.Command, io.Writer) error
	path func(string) string
	msg  string
}

var shells = map[string]shellHandler{
	"bash": {
		gen:  (*cobra.Command).GenBashCompletion,
		path: bashPath,
		msg:  "Restart your shell to apply.",
	},
	"zsh": {
		gen:  (*cobra.Command).GenZshCompletion,
		path: func(h string) string { return filepath.Join(h, ".zfunc", "_snirect") },
		msg:  zshMsg(),
	},
	"fish": {
		gen:  func(c *cobra.Command, w io.Writer) error { return c.GenFishCompletion(w, true) },
		path: func(h string) string { return filepath.Join(h, ".config", "fish", "completions", "snirect.fish") },
	},
	"powershell": {
		gen:  (*cobra.Command).GenPowerShellCompletionWithDesc,
		path: psPath,
		msg:  "Add the file to your $PROFILE to enable.",
	},
}

func runCompletion(cmd *cobra.Command, args []string) error {
	h := shells[args[0]]
	install, _ := cmd.Flags().GetBool("install")

	if !install {
		return h.gen(rootCmd, os.Stdout)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	target := h.path(home)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	var buf bytes.Buffer
	if err := h.gen(rootCmd, &buf); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}
	if err := os.WriteFile(target, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	fmt.Printf("Installed: %s\n", target)
	if h.msg != "" {
		fmt.Println(h.msg)
	}
	return nil
}

func bashPath(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, ".bash_completion.d", "snirect")
	}
	return filepath.Join(home, ".local", "share", "bash-completion", "completions", "snirect")
}

func psPath(home string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Scripts", "snirect-completion.ps1")
	}
	return filepath.Join(home, ".config", "powershell", "snirect-completion.ps1")
}

func zshMsg() string {
	if runtime.GOOS == "windows" {
		return "Restart your shell."
	}
	return "Restart your shell. Ensure ~/.zfunc is in fpath."
}

func init() {
	completionCmd.Flags().BoolP("install", "i", false, "Install completion script")
}
