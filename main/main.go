package main

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/update/all"
	v0_7_0 "github.com/skupperproject/skupper/pkg/update/v0.7.0"
	v0_8_0 "github.com/skupperproject/skupper/pkg/update/v0.8.0"
	v1_3_0 "github.com/skupperproject/skupper/pkg/update/v1.3.0"
)

func updatePlatform(platform types.Platform) error {
	p := &config.PlatformInfo{}
	return p.Update(platform)
}
func main() {
	config.PlatformConfigFile = "/tmp/platform.yml"

	// all update tasks must be registered by their respective platform client
	register()

	// then we simulate an update for the given platform supposedly running a given version
	kubeUpdate("0.5.0")
	kubeUpdate("1.0.0")
	kubeUpdate("1.3.0")

	podmanUpdate("0.7.0")
	podmanUpdate("0.8.0")
	podmanUpdate("1.2.0")
	podmanUpdate("1.3.0")
}

func register() {
	update.RegisterTask(&all.UpdateDeployments{})
	update.RegisterTask(&v0_8_0.AddConfigSync{})
	update.RegisterTask(&v0_8_0.MultiplePorts{})
	update.RegisterTask(&v1_3_0.AddVault{})
	update.RegisterTask(&v1_3_0.UpdateAddVflowCollectorKube{})
	update.RegisterTask(&v1_3_0.PodmanController{})
	update.RegisterTask(&v0_7_0.Claims{})
	update.RegisterTask(&all.UpdatePodmanContainers{})
	update.RegisterTask(&v0_8_0.PodmanSwitch{})
}

func kubeUpdate(siteVersion string) {
	fmt.Println("KUBERNETES UPDATE - From site version", siteVersion)
	fmt.Println("")
	updatePlatform(types.PlatformKubernetes)
	fmt.Println(update.Process(siteVersion))
	fmt.Println("")
	fmt.Println("----------------------------------------")
	fmt.Println("")
	fmt.Println("")
}

func podmanUpdate(siteVersion string) {
	fmt.Println("PODMAN UPDATE - From site version", siteVersion)
	fmt.Println("")
	updatePlatform(types.PlatformPodman)
	fmt.Println(update.Process(siteVersion))
	fmt.Println("")
	fmt.Println("----------------------------------------")
	fmt.Println("")
	fmt.Println("")
}
