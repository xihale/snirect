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
	Short: "Generate completion script",
	Long: `To load completions:

Bash:
  $ source <(snirect completion bash)

  # To load completions for each session, execute once:
  $ snirect completion bash --install

Zsh:
  $ source <(snirect completion zsh)

  # To load completions for each session, execute once:
  $ snirect completion zsh --install

Fish:
  $ snirect completion fish | source

  # To load completions for each session, execute once:
  $ snirect completion fish --install
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		shell := args[0]

		if installCompletion {
			installShellCompletion(shell, cmd)
		} else {
			switch shell {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		}
	},
}

func init() {
	completionCmd.Flags().BoolVarP(&installCompletion, "install", "i", false, "Automatically install completion script to user config")
	RootCmd.AddCommand(completionCmd)
}

func installShellCompletion(shell string, cmd *cobra.Command) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get user home dir: %v\n", err)
		os.Exit(1)
	}

	var path string
	var errGen error

	switch shell {
	case "bash":
		path = getBashCompletionPath(homeDir)
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err == nil {
			f, errCreate := os.Create(path)
			if errCreate == nil {
				defer f.Close()
				errGen = cmd.Root().GenBashCompletion(f)
			} else {
				err = errCreate
			}
		}
	case "zsh":
		path = getZshCompletionPath(homeDir)
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err == nil {
			f, errCreate := os.Create(path)
			if errCreate == nil {
				defer f.Close()
				errGen = cmd.Root().GenZshCompletion(f)
				if runtime.GOOS != "windows" {
					fmt.Println("Note: Ensure ~/.zfunc is in your fpath in .zshrc:")
					fmt.Println("      fpath+=~/.zfunc; autoload -U compinit; compinit")
				}
			} else {
				err = errCreate
			}
		}
	case "fish":
		path = getFishCompletionPath(homeDir)
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err == nil {
			f, errCreate := os.Create(path)
			if errCreate == nil {
				defer f.Close()
				errGen = cmd.Root().GenFishCompletion(f, true)
			} else {
				err = errCreate
			}
		}
	case "powershell":
		path = getPowerShellCompletionPath(homeDir)
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err == nil {
			f, errCreate := os.Create(path)
			if errCreate == nil {
				defer f.Close()
				errGen = cmd.Root().GenPowerShellCompletionWithDesc(f)
				fmt.Printf("To enable PowerShell completions, add this line to your $PROFILE:\n")
				fmt.Printf(". %s\n", path)
			} else {
				err = errCreate
			}
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create completion file at %s: %v\n", path, err)
		os.Exit(1)
	}
	if errGen != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate completion content: %v\n", errGen)
		os.Exit(1)
	}

	fmt.Printf("Completion script installed to: %s\n", path)
	if shell == "bash" || shell == "zsh" {
		fmt.Println("Please restart your shell for changes to take effect.")
	}
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
