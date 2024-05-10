/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

var (
	siteCreateLong = `Setup a router and other supporting objects to provide a functional skupper
installation that can then be connected to other skupper installations.`
	siteCreateExample = `skupper site create --name site1 --enable-collector --enable-console
skupper site create --ingress route`
)

type CreateFlags struct {
	name                     string
	ingress                  string
	ingressHost              string
	labels                   []string
	routerMode               string
	routerLogging            string
	enableConsole            bool
	enableFlowCollector      bool
	consoleAuth              string
	consoleUser              string
	consolePassword          string
	routerCPULimit           string
	routerMemoryLimit        string
	controllerCPULimit       string
	controllerMemoryLimit    string
	flowCollectorCPULimit    string
	flowCollectorMemoryLimit string
	prometheusCPULimit       string
	prometheusMemoryLimit    string
}

type CmdSiteCreate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
	flags    CreateFlags
	options  map[string]string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	options := make(map[string]string)
	skupperCmd := CmdSiteCreate{options: options, flags: CreateFlags{}}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "Create a new site",
		Long:    siteCreateLong,
		Example: siteCreateExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateFlags())
			utils.HandleError(skupperCmd.FlagsToOptions())
			utils.HandleError(skupperCmd.Run())
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if ok := skupperCmd.WaitUntilReady(); ok {
				fmt.Printf("Site \"%s\" is ready\n", skupperCmd.options["name"])
			} else {
				fmt.Printf("Site \"%s\" not ready yet, check the logs for more information\n", skupperCmd.options["name"])
			}

		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdSiteCreate) AddFlags() {
	//Generic
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.name, "name", "", "Provide a specific name for this skupper installation (by default the same as in the namespace)")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.ingress, "ingress", "none", "Setup Skupper ingress to one of: [route|loadbalancer|nodeport|nginx-ingress-v1|contour-http-proxy|ingress|none]")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.ingressHost, "ingress-host", "", "Hostname or alias by which the ingress route or proxy can be reached")
	cmd.CobraCmd.Flags().StringSliceVar(&cmd.flags.labels, "labels", []string{}, "Labels to add to resources created by skupper")

	//Router
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.routerMode, "router-mode", "interior", "Skupper router-mode")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.routerLogging, "router-logging", "info", "Logging settings for router. On of [trace|debug|info|notice|warning|error]")

	//Console and Flow collector
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableFlowCollector, "enable-flow-collector", false, "Enable cross-site flow collection for the application network")
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableConsole, "enable-console", false, "Enable skupper console must be used in conjunction with '--enable-flow-collector' flag")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.consoleAuth, "console-auth", "internal", "Authentication mode for console(s). One of: [openshift|internal|unsecured]")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.consoleUser, "console-user", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.consolePassword, "console-password", "", "Skupper console password. Valid only when --console-auth=internal")

	//Setting limits
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.routerCPULimit, "router-cpu-limit", "", "CPU limit for router pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.routerMemoryLimit, "router-memory-limit", "", "Memory limit for router pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.controllerCPULimit, "controller-cpu-limit", "", "CPU limit for controller pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.controllerMemoryLimit, "controller-memory-limit", "", "Memory limit for controller pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.flowCollectorCPULimit, "flow-collector-cpu-limit", "", "CPU limit for flow collector pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.flowCollectorMemoryLimit, "flow-collector-memory-limit", "", "Memory limit for flow collector pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.prometheusCPULimit, "prometheus-cpu-limit", "", "CPU limit for prometheus pods")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.prometheusMemoryLimit, "prometheus-memory-limit", "", "Memory limit for flow prometheus pods")

}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient("", "", "")
	utils.HandleError(err)

	cmd.client = cli
}

func (cmd *CmdSiteCreate) ValidateFlags() []error {

	var validationErrors []error
	stringValidator := validator.NewStringValidator()

	ok, err := stringValidator.Evaluate(cmd.flags.name)
	if !ok {
		validationErrors = append(validationErrors, err)
	}

	if cmd.flags.enableConsole && !cmd.flags.enableFlowCollector {
		validationErrors = append(validationErrors, fmt.Errorf("the --enable-flow-collector option must be used with the --enable-console option"))
	}

	if cmd.flags.consoleAuth != "internal" && (len(cmd.flags.consoleUser) > 0 || len(cmd.flags.consolePassword) > 0) {
		validationErrors = append(validationErrors, fmt.Errorf("for the console to work with this user or password, the --console-auth option must be set to internal"))
	}

	return validationErrors
}

func (cmd *CmdSiteCreate) FlagsToOptions() error {

	options := make(map[string]string)

	options["name"] = cmd.flags.name
	options["ingress"] = cmd.flags.ingress

	if cmd.flags.ingressHost != "" {
		options["name"] = cmd.flags.ingressHost
	}

	if len(cmd.flags.labels) > 0 {
		options["ingress"] = strings.Join(cmd.flags.labels, "")
	}

	options["router-mode"] = cmd.flags.routerMode
	options["router-logging"] = cmd.flags.routerLogging

	options["enable-flow-collector"] = strconv.FormatBool(cmd.flags.enableFlowCollector)
	options["enable-console"] = strconv.FormatBool(cmd.flags.enableConsole)
	options["console-auth"] = cmd.flags.consoleAuth
	options["console-user"] = cmd.flags.consoleUser
	options["console-password"] = cmd.flags.consolePassword

	if cmd.flags.routerCPULimit != "" {
		options["router-cpu-limit"] = cmd.flags.routerCPULimit
	}
	if cmd.flags.routerMemoryLimit != "" {
		options["router-memory-limit"] = cmd.flags.routerMemoryLimit
	}
	if cmd.flags.controllerCPULimit != "" {
		options["controller-cpu-limit"] = cmd.flags.controllerCPULimit
	}
	if cmd.flags.controllerMemoryLimit != "" {
		options["controller-memory-limit"] = cmd.flags.controllerMemoryLimit
	}
	if cmd.flags.flowCollectorCPULimit != "" {
		options["flow-collector-cpu-limit"] = cmd.flags.flowCollectorCPULimit
	}

	if cmd.flags.flowCollectorMemoryLimit != "" {
		options["flow-collector-memory-limit"] = cmd.flags.flowCollectorMemoryLimit
	}

	if cmd.flags.prometheusCPULimit != "" {
		options["prometheus-cpu-limit"] = cmd.flags.prometheusCPULimit
	}

	if cmd.flags.prometheusMemoryLimit != "" {
		options["prometheus-memory-limit"] = cmd.flags.prometheusMemoryLimit
	}

	cmd.options = options

	return nil
}

func (cmd *CmdSiteCreate) Run() error {

	siteName := cmd.options["name"]
	if siteName == "" {
		siteName = cmd.client.Namespace
	}

	resource := v1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{Name: siteName},
		Spec: v1alpha1.SiteSpec{
			Settings: cmd.options,
		},
	}

	_, err := cmd.client.GetSkupperClient().SkupperV1alpha1().Sites(cmd.client.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	utils.HandleError(err)
	return nil
}

func (cmd *CmdSiteCreate) WaitUntilReady() bool {

	err := utils.NewSpinner("Waiting for site...", 5, func() error {

		resource, err := cmd.client.GetSkupperClient().SkupperV1alpha1().Sites(cmd.client.Namespace).Get(context.TODO(), cmd.options["name"], metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return false
	}

	return true
}
