package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
		path = filepath.Join(homeDir, ".local", "share", "bash-completion", "completions", "snirect")
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
		path = filepath.Join(homeDir, ".zfunc", "_snirect")
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err == nil {
			f, errCreate := os.Create(path)
			if errCreate == nil {
				defer f.Close()
				errGen = cmd.Root().GenZshCompletion(f)
				fmt.Println("Note: Ensure ~/.zfunc is in your fpath in .zshrc:")
				fmt.Println("      fpath+=~/.zfunc; autoload -U compinit; compinit")
			} else {
				err = errCreate
			}
		}
	case "fish":
		path = filepath.Join(homeDir, ".config", "fish", "completions", "snirect.fish")
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
		fmt.Println("Automatic installation for PowerShell is not currently supported.")
		return
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
