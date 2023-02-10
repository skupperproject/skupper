package qdr

import (
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/api/types"
)

func RouterLogConfigToString(config []types.RouterLogConfig) string {
	items := []string{}
	for _, l := range config {
		if l.Module != "" && l.Level != "" {
			items = append(items, l.Module+":"+l.Level)
		} else if l.Level != "" {
			items = append(items, l.Level)
		}
	}
	return strings.Join(items, ",")
}

func ConfigureRouterLogging(routerConfig *RouterConfig, logConfig []types.RouterLogConfig) bool {
	levels := map[string]string{}
	for _, l := range logConfig {
		levels[l.Module] = l.Level
	}
	return routerConfig.SetLogLevels(levels)
}

func ParseRouterLogConfig(config string) ([]types.RouterLogConfig, error) {
	items := strings.Split(config, ",")
	parsed := []types.RouterLogConfig{}
	for _, item := range items {
		parts := strings.Split(item, ":")
		var mod string
		var level string
		if len(parts) > 1 {
			mod = parts[0]
			level = parts[1]
		} else if len(parts) > 0 {
			level = parts[0]
		}
		err := checkLoggingModule(mod)
		if err != nil {
			return nil, err
		}
		err = checkLoggingLevel(level)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, types.RouterLogConfig{
			Module: mod,
			Level:  level,
		})
	}
	return parsed, nil
}

var LoggingModules []string = []string{
	"", /*implies DEFAULT*/
	"ROUTER",
	"ROUTER_CORE",
	"ROUTER_HELLO",
	"ROUTER_LS",
	"ROUTER_MA",
	"MESSAGE",
	"SERVER",
	"AGENT",
	"AUTHSERVICE",
	"CONTAINER",
	"ERROR",
	"POLICY",
	"HTTP",
	"CONN_MGR",
	"PYTHON",
	"PROTOCOL",
	"TCP_ADAPTOR",
	"HTTP_ADAPTOR",
	"DEFAULT",
}
var LoggingLevels []string = []string{
	"trace",
	"debug",
	"info",
	"notice",
	"warning",
	"error",
	"critical",
	"trace+",
	"debug+",
	"info+",
	"notice+",
	"warning+",
	"error+",
	"critical+",
}

func checkLoggingModule(mod string) error {
	for _, m := range LoggingModules {
		if mod == m {
			return nil
		}
	}
	return fmt.Errorf("Invalid logging module for router: %s", mod)
}

func checkLoggingLevel(level string) error {
	for _, l := range LoggingLevels {
		if level == l {
			return nil
		}
	}
	return fmt.Errorf("Invalid logging level for router: %s", level)
}
