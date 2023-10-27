// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugincontainer_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/go-secure-stdlib/plugincontainer"
)

const (
	runtimeRunc  = "runc"
	runtimeRunsc = "runsc"
)

type matrixInput struct {
	containerRuntime string
	rootlessEngine   bool
	rootlessUser     bool
	mlock            bool
}

func (m matrixInput) String() string {
	var s string
	if m.rootlessEngine {
		s = "rootless_"
	}
	s += "docker"
	s += ":" + m.containerRuntime
	if m.rootlessUser {
		s += ":" + "nonroot"
	}
	if m.mlock {
		s += ":" + "mlock"
	}
	return s
}

func TestCompatibilityMatrix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Only linux is supported for now")
	}

	runCmd(t, "go", "build", "-o=examples/container/go-plugin-counter", "./examples/container/plugin-counter")

	for _, runtime := range []string{runtimeRunc, runtimeRunsc} {
		for _, rootlessEngine := range []bool{true, false} {
			for _, rootlessUser := range []bool{true, false} {
				for _, mlock := range []bool{true, false} {
					i := matrixInput{
						containerRuntime: runtime,
						rootlessEngine:   rootlessEngine,
						rootlessUser:     rootlessUser,
						mlock:            mlock,
					}
					t.Run(i.String(), func(t *testing.T) {
						runExamplePlugin(t, i)
					})
				}
			}
		}
	}
}

func setDockerHost(t *testing.T, rootlessEngine bool) {
	var socketFile string
	switch {
	case !rootlessEngine:
		socketFile = "/var/run/docker.sock"
	case rootlessEngine:
		socketFile = fmt.Sprintf("/run/user/%d/docker.sock", os.Getuid())
	}
	if _, err := os.Stat(socketFile); err != nil {
		t.Fatal("Did not find expected socket file:", err)
	}
	t.Setenv("DOCKER_HOST", "unix://"+socketFile)
}

func runExamplePlugin(t *testing.T, i matrixInput) {
	setDockerHost(t, i.rootlessEngine)

	imageRef := goPluginCounterImage
	target := "root"
	if i.rootlessUser {
		imageRef += ":nonroot"
		if i.mlock {
			target = "nonroot-mlock"
		} else {
			target = "nonroot"
		}
	}
	runCmd(t, "docker", "build", "--tag="+imageRef, "--target="+target, "--file=examples/container/Dockerfile", "examples/container")

	cfg := &plugincontainer.Config{
		Image:    goPluginCounterImage,
		Runtime:  i.containerRuntime,
		GroupAdd: os.Getgid(),
		Debug:    true,

		CapIPCLock: i.mlock,
	}
	if i.mlock {
		cfg.Env = append(cfg.Env, "MLOCK=true")
	}
	if i.rootlessUser {
		cfg.Tag = "nonroot"
	}
	exerciseExamplePlugin(t, cfg)
}