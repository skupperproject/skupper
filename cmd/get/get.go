package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func path(arg string) (string, error) {
	if arg == "" {
		return "version", nil
	}
	if arg == "events" || arg == "version" || arg == "sites" || arg == "services" {
		return arg, nil
	}
	return "", fmt.Errorf("Invalid argument: %s", arg)
}

func get(arg string, output string) error {
	path, err := path(arg)
	if err != nil {
		return err
	}

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
var rootCmd = &cobra.Command{
	Use:   "get [events|sites|services|version]",
	Short: "A simple tool to retrieve information via a HTTP GET request",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		if err := get(path, output); err != nil {
			fmt.Println(err)
		}
	},
}

func main() {
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "The output format to use (one of json or text)")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
