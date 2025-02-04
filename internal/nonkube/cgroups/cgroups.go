package cgroups

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func LoadCgroupControllers() CgroupControllers {
	c := CgroupControllers{}
	// Cgroups version
	cgroup2, err := isCgroup2()
	if err != nil {
		fmt.Printf("warning: reading cgroups mode: %v\n", err)
	}
	// List all available controllers
	controllers, err := AvailableControllers(cgroup2)
	if err != nil {
		fmt.Printf("warning: error finding available controllers: %v\n", err)
	}
	c.controllers = controllers
	return c
}

type CgroupControllers struct {
	controllers []string
}

func (c *CgroupControllers) hasValue(val string) bool {
	for _, v := range c.controllers {
		if v == val {
			return true
		}
	}
	return false
}

func (c *CgroupControllers) HasCPU() bool {
	return c.hasValue("cpu")
}

func (c *CgroupControllers) HasMemory() bool {
	return c.hasValue("memory")
}

// AvailableControllers retrieves a list of available cgroup controllers
// based on current user and cgroup version.
// This is a simplification of github.com/containers/common/pkg/cgroups
// for internal purposes.
func AvailableControllers(cgroup2 bool) ([]string, error) {
	var controllers []string
	isRootless := os.Geteuid() != 0
	const cgroupRoot = "/sys/fs/cgroup"
	if cgroup2 {
		controllersFile := cgroupRoot + "/cgroup.controllers"
		if isRootless {
			userSlice, err := getCgroupPathForCurrentProcess()
			if err != nil {
				return controllers, err
			}
			// userSlice already contains '/' so not adding here
			basePath := cgroupRoot + userSlice
			controllersFile = basePath + "/cgroup.controllers"
		}
		controllersFileBytes, err := os.ReadFile(controllersFile)
		if err != nil {
			return nil, fmt.Errorf("failed while reading controllers for cgroup v2: %w", err)
		}
		for _, controllerName := range strings.Fields(string(controllersFileBytes)) {
			controllers = append(controllers, controllerName)
		}
		return controllers, nil
	}
	subsystems, _ := cgroupV1GetAllSubsystems()
	if isRootless {
		return controllers, nil
	}
	for _, name := range subsystems {
		_, err := os.Stat(cgroupRoot + "/" + name)
		if err != nil {
			continue
		}
		controllers = append(controllers, name)
	}
	return controllers, nil
}

func isCgroup2() (bool, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/sys/fs/cgroup", &st); err != nil {
		return false, err
	} else {
		return st.Type == unix.CGROUP2_SUPER_MAGIC, nil
	}
}

func getCgroupPathForCurrentProcess() (string, error) {
	path := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cgroupPath := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		procEntries := strings.SplitN(text, "::", 2)
		// set process cgroupPath only if entry is valid
		if len(procEntries) > 1 {
			cgroupPath = procEntries[1]
		}
	}
	if err := s.Err(); err != nil {
		return cgroupPath, err
	}
	return cgroupPath, nil
}

func cgroupV1GetAllSubsystems() ([]string, error) {
	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	subsystems := []string{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		if text[0] != '#' {
			parts := strings.Fields(text)
			if len(parts) >= 4 && parts[3] != "0" {
				subsystems = append(subsystems, parts[0])
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return subsystems, nil
}
