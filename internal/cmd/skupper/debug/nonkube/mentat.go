package nonkube

import (
	"fmt"
	"os"

	"github.com/mgoulish/mentat-go-2/pkg/mentat"
	"github.com/spf13/cobra"
)

type CmdDebugMentat struct {
	CobraCmd *cobra.Command
	dumpFile string
	timeFlag string
}

func NewCmdDebugMentat() *CmdDebugMentat {
	return &CmdDebugMentat{}
}

func (cmd *CmdDebugMentat) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.CobraCmd = cobraCommand

	// Handle filename argument
	if len(args) > 0 {
		cmd.dumpFile = args[0]
	}

	// Read the --time flag
	cmd.timeFlag, _ = cobraCommand.Flags().GetString("time")
}

func (cmd *CmdDebugMentat) ValidateInput(args []string) error {
	return nil
}

func (cmd *CmdDebugMentat) InputToOptions() {}

func (cmd *CmdDebugMentat) Run() error {
	if cmd.dumpFile == "" {
		fmt.Println("No dump file specified.")
		fmt.Println("Usage:")
		fmt.Println("  skupper debug mentat <dumpfile.tar.gz>")
		fmt.Println("  skupper debug mentat <dumpfile.tar.gz> --time \"2025-05-11 14:30:00\"")
		fmt.Println("\nFirst create a dump:")
		fmt.Println("  skupper debug dump my-dump.tar.gz")
		return nil
	}

	if _, err := os.Stat(cmd.dumpFile); os.IsNotExist(err) {
		return fmt.Errorf("dump file not found: %s", cmd.dumpFile)
	}

	fmt.Printf("🔍 Running mentat analysis on %s...\n", cmd.dumpFile)

	// === This is the key logic you asked for ===
	if cmd.timeFlag != "" {
		fmt.Printf("⏰ Checking connectivity at specific time: %s\n", cmd.timeFlag)
		return mentat.CheckAtTime(cmd.dumpFile, cmd.timeFlag) // ← you will implement this
	}

	// Default behavior
	return mentat.DoEverything(cmd.dumpFile)
}

func (cmd *CmdDebugMentat) WaitUntil() error {
	return nil
}
