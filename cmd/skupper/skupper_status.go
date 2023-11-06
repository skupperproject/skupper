package main

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
)

type PlatformSupport struct {
	supportType string
	supportName string
}

type StatusData struct {
	enabledIn           PlatformSupport
	mode                string
	siteName            string
	policies            string
	status              *string
	warnings            []string
	totalConnections    int
	directConnections   int
	indirectConnections int
	exposedServices     int
	consoleUrl          string
	credentials         PlatformSupport
}

func PrintStatus(data StatusData) error {

	enabledIn := fmt.Sprintf("%q", data.enabledIn.supportName)
	if data.enabledIn.supportType == "kubernetes" {
		enabledIn = fmt.Sprintf("namespace %q", data.enabledIn.supportName)
	}

	siteName := ""
	if data.siteName != "" && data.siteName != data.enabledIn.supportName {
		siteName = siteName + fmt.Sprintf(" with site name %q", data.siteName)
	}
	policyStr := ""

	if data.policies == "enabled" {
		policyStr = " (with policies)"
	}

	fmt.Printf("Skupper is enabled for %s%s%s.", enabledIn, siteName, policyStr)
	if data.status != nil {
		fmt.Printf(" Status pending...")
	} else {
		if len(data.warnings) > 0 {
			for _, w := range data.warnings {
				fmt.Printf("Warning: %s", w)
				fmt.Println()
			}
		}
		if data.totalConnections == 0 {
			fmt.Printf(" It is not connected to any other sites.")
		} else if data.totalConnections == 1 {
			fmt.Printf(" It is connected to 1 other site.")
		} else if data.totalConnections == data.directConnections {
			fmt.Printf(" It is connected to %d other sites.", data.totalConnections)
		} else {
			fmt.Printf(" It is connected to %d other sites (%d indirectly).", data.totalConnections, data.indirectConnections)
		}
	}
	if data.exposedServices == 0 {
		fmt.Printf(" It has no exposed services.")
	} else if data.exposedServices == 1 {
		fmt.Printf(" It has 1 exposed service.")
	} else {
		fmt.Printf(" It has %d exposed services.", data.exposedServices)
	}
	fmt.Println()

	if len(data.consoleUrl) > 0 {
		fmt.Println("The site console url is: ", data.consoleUrl)
		if len(data.credentials.supportName) > 0 {
			fmt.Printf("The credentials for internal console-auth mode are held in %s: %s", data.credentials.supportType, data.credentials.supportName)
			fmt.Println()
		}
	}

	return nil
}

func PrintVerboseStatus(data StatusData) error {
	writer := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintf(writer, "%s:\t %s \n", data.enabledIn.supportType, data.enabledIn.supportName)
	routerMode := "interior"
	if len(data.mode) > 0 {
		routerMode = data.mode
	}
	fmt.Fprintf(writer, "%s:\t %s \n", "mode", routerMode)
	fmt.Fprintf(writer, "%s:\t %s \n", "site name", data.siteName)
	fmt.Fprintf(writer, "%s:\t %s \n", "policies", data.policies)

	if data.status != nil {
		fmt.Fprintf(writer, "%s:\t %s \n", "status", *data.status)
	}

	for index, w := range data.warnings {
		warningIndex := fmt.Sprintf("warning %d", index)
		fmt.Fprintf(writer, "%s:\t %s \n", warningIndex, w)
	}

	fmt.Fprintf(writer, "%s:\t %s \n", "total connections", strconv.Itoa(data.totalConnections))
	fmt.Fprintf(writer, "%s:\t %s \n", "direct connections", strconv.Itoa(data.directConnections))
	fmt.Fprintf(writer, "%s:\t %s \n", "indirect connections", strconv.Itoa(data.indirectConnections))

	fmt.Fprintf(writer, "%s:\t %s \n", "exposed services", strconv.Itoa(data.exposedServices))

	if len(data.consoleUrl) > 0 {
		fmt.Fprintf(writer, "%s:\t %s \n", "site console url", data.consoleUrl)
	}

	if len(data.credentials.supportName) > 0 {
		fmt.Fprintf(writer, "%s:\t %s \n", "credentials", data.credentials.supportName)
	}

	err := writer.Flush()
	if err != nil {
		return err
	}

	return nil
}
