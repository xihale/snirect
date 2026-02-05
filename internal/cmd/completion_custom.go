package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var installCompletion bool

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate and install shell completion scripts for snirect.

Temporary usage (current shell only):
  Bash:   source <(snirect completion bash)
  Zsh:    source <(snirect completion zsh)
  Fish:   snirect completion fish | source

Permanent installation (automatic on new shells):
  snirect completion bash --install    # Install for bash
  snirect completion zsh --install     # Install for zsh
  snirect completion fish --install    # Install for fish

After installation, restart your shell or run the temporary usage command.`,
	Example: `  snirect completion bash          # Print bash completions
  snirect completion zsh --install   # Install zsh completions permanently
  snirect completion fish | source   # Load fish completions now`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]

		if installCompletion {
			return installShellCompletion(shell, cmd)
		} else {
			return dumpCompletion(shell)
		}
	},
}

func dumpCompletion(shell string) error {
	// Try embedded first
	data, err := completionsFS.ReadFile("completions/" + shell)
	if err == nil && len(data) > 0 {
		_, err = os.Stdout.Write(data)
		return err
	}

	// Fallback to dynamic generation (needed during build/bootstrap)
	switch shell {
	case "bash":
		return GetRootCmd().GenBashCompletion(os.Stdout)
	case "zsh":
		return GetRootCmd().GenZshCompletion(os.Stdout)
	case "fish":
		return GetRootCmd().GenFishCompletion(os.Stdout, true)
	case "powershell":
		return GetRootCmd().GenPowerShellCompletionWithDesc(os.Stdout)
	}
	return fmt.Errorf("unknown shell: %s", shell)
}

func init() {
	completionCmd.Flags().BoolVarP(&installCompletion, "install", "i", false, "Automatically install completion script to user config")
	RootCmd.AddCommand(completionCmd)
}

func installShellCompletion(shell string, cmd *cobra.Command) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	var path string

	switch shell {
	case "bash":
		path = getBashCompletionPath(homeDir)
		err = writeCompletionFile(path, shell)
	case "zsh":
		path = getZshCompletionPath(homeDir)
		err = writeCompletionFile(path, shell)
		if err == nil && runtime.GOOS != "windows" {
			fmt.Println("Note: Ensure ~/.zfunc is in your fpath in .zshrc:")
			fmt.Println("      fpath+=~/.zfunc; autoload -U compinit; compinit")
		}
	case "fish":
		path = getFishCompletionPath(homeDir)
		err = writeCompletionFile(path, shell)
	case "powershell":
		path = getPowerShellCompletionPath(homeDir)
		err = writeCompletionFile(path, shell)
		if err == nil {
			fmt.Printf("To enable PowerShell completions, add this line to your $PROFILE:\n")
			fmt.Printf(". %s\n", path)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to install completion: %w", err)
	}

	fmt.Printf("Completion script installed to: %s\n", path)
	if shell == "bash" || shell == "zsh" {
		fmt.Println("Please restart your shell for changes to take effect.")
	}
	return nil
}

func writeCompletionFile(path, shell string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := completionsFS.ReadFile("completions/" + shell)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func getBashCompletionPath(homeDir string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir, ".bash_completion.d", "snirect")
	}
	return filepath.Join(homeDir, ".local", "share", "bash-completion", "completions", "snirect")
}

func getZshCompletionPath(homeDir string) string {
	return filepath.Join(homeDir, ".zfunc", "_snirect")
}

func getFishCompletionPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "fish", "completions", "snirect.fish")
}

func getPowerShellCompletionPath(homeDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(homeDir, "Documents", "PowerShell", "Scripts", "snirect-completion.ps1")
	}
	return filepath.Join(homeDir, ".config", "powershell", "snirect-completion.ps1")
}
