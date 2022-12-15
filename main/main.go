package main

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/main/kube"
	"github.com/skupperproject/skupper/main/kube/all"
	v0_7_0 "github.com/skupperproject/skupper/main/kube/v0.7.0"
	v0_8_0 "github.com/skupperproject/skupper/main/kube/v0.8.0"
	v1_3_0 "github.com/skupperproject/skupper/main/kube/v1.3.0"
	podman "github.com/skupperproject/skupper/main/podman"
	podmanall "github.com/skupperproject/skupper/main/podman/all"
	podmanv0_8_0 "github.com/skupperproject/skupper/main/podman/v0.8.0"
	podmanv1_3_0 "github.com/skupperproject/skupper/main/podman/v1.3.0"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/update"
)

func updatePlatform(platform types.Platform) error {
	p := &config.PlatformInfo{}
	return p.Update(platform)
}
func main() {
	config.PlatformConfigFile = "/tmp/platform.yml"

	// Simulating what the kube client would do for an update
	simulateKubeClient()

	// Simulating what the podman client would do for an update
	simulatePodmanClient()

}

// simulateKubeClient could be seen as the update done through cmd/skupper/skupper_kube_site
func simulateKubeClient() {

	// all update tasks must be registered by their respective platform client
	registerKubeTasks()

	// then we simulate an update for the given platform supposedly running a given version
	kubeUpdate("0.5.0")
	kubeUpdate("1.0.0")
	kubeUpdate("1.3.0")

}

func simulatePodmanClient() {

	// all update tasks must be registered by their respective platform client
	registerPodmanTasks()

	// then we simulate an update for the given platform supposedly running a given version
	podmanUpdate("0.7.0")
	podmanUpdate("0.8.0")
	podmanUpdate("1.2.0")
	podmanUpdate("1.3.0")

}

func registerKubeTasks() {
	cli, _ := client.NewClient("", "", "")
	common := &kube.KubeTask{
		Cli: cli,
	}

	update.RegisterTask(&all.UpdateDeployments{Common: common})
	update.RegisterTask(&v0_8_0.AddConfigSync{Common: common})
	update.RegisterTask(&v0_8_0.MultiplePorts{Common: common})
	update.RegisterTask(&v1_3_0.AddVaultKube{Common: common})
	update.RegisterTask(&v1_3_0.UpdateAddVflowCollectorKube{Common: common})
	update.RegisterTask(&v0_7_0.Claims{Common: common})
}

func registerPodmanTasks() {
	cli, _ := clientpodman.NewPodmanClient("", "")
	common := &podman.PodmanTask{
		Cli: cli,
	}
	update.RegisterTask(&podmanv1_3_0.AddVaultPodman{Common: common})
	update.RegisterTask(&podmanv1_3_0.PodmanController{Common: common})
	update.RegisterTask(&podmanall.UpdatePodmanContainers{Common: common})
	update.RegisterTask(&podmanv0_8_0.PodmanSwitch{Common: common})
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
