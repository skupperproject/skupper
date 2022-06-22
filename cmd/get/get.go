package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

func get(path string, output string) error {
	var resp *http.Response
	var err error

	err = utils.Retry(time.Second, 30, func() (bool, error) {
		url := "http://localhost:8181/" + path
		if output == "json" {
			url += "?output=json"
		}
		resp, err = http.Get(url)
		if err != nil {
			if strings.Contains(err.Error(), "connect: connection refused") {
				return false, nil
			}
			return true, err
		}
		return true, nil
	})

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.Status != "200 OK" {
		fmt.Println("Response status:", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return scanner.Err()
}

var output string

func simplePathCommand(path string, description string) *cobra.Command {
	return &cobra.Command{
		Use:   path,
		Short: description,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return get(path, output)
		},
	}
}

func policyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use:   "policies",
		Short: "Validates existing policies",
	}
	simplePathPolicyCommand := func(pathCmd string, args []string, description string) {
		commands := []string{pathCmd}
		cobraArgs := cobra.NoArgs
		if len(args) > 0 {
			commands = append(commands, args...)
			cobraArgs = cobra.ExactArgs(len(args))
		}
		policyCmd.AddCommand(&cobra.Command{
			Use:   strings.Join(commands, " "),
			Short: description,
			Args:  cobraArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				path := fmt.Sprintf("policy/%s", pathCmd)
				for i, _ := range args {
					path = path + "/" + args[i]
				}
				return get(path, output)
			},
		})
	}
	simplePathPolicyCommand("incominglink", nil, "Validates if incoming links can be created")
	simplePathPolicyCommand("outgoinglink", []string{"hostname"}, "Validates if an outgoing link to the given hostname is allowed")
	simplePathPolicyCommand("expose", []string{"target-type", "target-name"}, "Validates if the given resource can be exposed")
	simplePathPolicyCommand("service", []string{"name"}, "Validates if service can be created or imported")
	return policyCmd
}

func main() {
	var rootCmd = &cobra.Command{Use: "get"}

	rootCmd.AddCommand(simplePathCommand("events", "Shows most recent events"))
	rootCmd.AddCommand(simplePathCommand("version", "Shows version information"))
	rootCmd.AddCommand(simplePathCommand("sites", "Shows connected sites"))
	rootCmd.AddCommand(simplePathCommand("services", "Shows exposed services"))

	rootCmd.AddCommand(&cobra.Command{
		Use:   "servicecheck <address>",
		Short: "Check configuration for an exposed service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "servicecheck/" + args[0]
			return get(path, output)
		},
	})

	rootCmd.AddCommand(policyCmd())

	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "The output format to use (one of json or text)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
