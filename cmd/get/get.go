package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func get(path string, output string) error {
	url := "http://localhost:8181/" + path
	if output == "json" {
		url += "?output=json"
	}
	resp, err := http.Get(url)
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

	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "The output format to use (one of json or text)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
