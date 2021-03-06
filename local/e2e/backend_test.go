/*
   Copyright 2020 Docker Compose CLI authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"

	. "github.com/docker/compose-cli/tests/framework"
)

var binDir string

func TestMain(m *testing.M) {
	p, cleanup, err := SetupExistingCLI()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	binDir = p
	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}

func TestLocalBackendRun(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	c.RunDockerCmd("context", "create", "local", "test-context").Assert(t, icmd.Success)
	c.RunDockerCmd("context", "use", "test-context").Assert(t, icmd.Success)

	t.Run("run", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("run", "-d", "nginx")
		containerName := strings.TrimSpace(res.Combined())
		t.Cleanup(func() {
			_ = c.RunDockerOrExitError("rm", "-f", containerName)
		})
		res = c.RunDockerCmd("inspect", containerName)
		res.Assert(t, icmd.Expected{Out: `"Status": "running"`})
	})

	t.Run("run rm", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("run", "--rm", "-d", "nginx")
		containerName := strings.TrimSpace(res.Combined())
		t.Cleanup(func() {
			_ = c.RunDockerOrExitError("rm", "-f", containerName)
		})
		_ = c.RunDockerCmd("stop", containerName)
		checkRemoved := func(t poll.LogT) poll.Result {
			res = c.RunDockerOrExitError("inspect", containerName)
			if res.ExitCode == 1 && strings.Contains(res.Stderr(), "No such container") {
				return poll.Success()
			}
			return poll.Continue("waiting for container to be removed")
		}
		poll.WaitOn(t, checkRemoved, poll.WithDelay(1*time.Second), poll.WithTimeout(10*time.Second))
	})

	t.Run("run with ports", func(t *testing.T) {
		res := c.RunDockerCmd("run", "-d", "-p", "8080:80", "nginx")
		containerName := strings.TrimSpace(res.Combined())
		t.Cleanup(func() {
			_ = c.RunDockerOrExitError("rm", "-f", containerName)
		})
		res = c.RunDockerCmd("inspect", containerName)
		res.Assert(t, icmd.Expected{Out: `"Status": "running"`})
		res = c.RunDockerCmd("ps")
		res.Assert(t, icmd.Expected{Out: "0.0.0.0:8080->80/tcp"})
	})

	t.Run("run with volume", func(t *testing.T) {
		t.Parallel()
		t.Cleanup(func() {
			_ = c.RunDockerOrExitError("volume", "rm", "local-test")
		})
		c.RunDockerCmd("volume", "create", "local-test")
		c.RunDockerCmd("run", "--rm", "-d", "--volume", "local-test:/data", "alpine", "sh", "-c", `echo "testdata" > /data/test`)
		// FIXME: Remove sleep when race to attach to dead container is fixed
		res := c.RunDockerOrExitError("run", "--rm", "--volume", "local-test:/data", "alpine", "sh", "-c", "cat /data/test && sleep 1")
		res.Assert(t, icmd.Expected{Out: "testdata"})
	})

	t.Run("inspect not found", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerOrExitError("inspect", "nonexistentcontainer")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Error: No such container: nonexistentcontainer",
		})
	})
}

func TestLocalBackendVolumes(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	c.RunDockerCmd("context", "create", "local", "test-context").Assert(t, icmd.Success)
	c.RunDockerCmd("context", "use", "test-context").Assert(t, icmd.Success)

	t.Run("volume crud", func(t *testing.T) {
		t.Parallel()
		name := "crud"
		t.Cleanup(func() {
			_ = c.RunDockerOrExitError("volume", "rm", name)
		})
		res := c.RunDockerCmd("volume", "create", name)
		res.Assert(t, icmd.Expected{Out: name})
		res = c.RunDockerCmd("volume", "ls")
		res.Assert(t, icmd.Expected{Out: name})
		res = c.RunDockerCmd("volume", "inspect", name)
		res.Assert(t, icmd.Expected{Out: fmt.Sprintf(`"ID": "%s"`, name)})
		res = c.RunDockerCmd("volume", "rm", name)
		res.Assert(t, icmd.Expected{Out: name})
		res = c.RunDockerOrExitError("volume", "inspect", name)
		res.Assert(t, icmd.Expected{ExitCode: 1})
	})
}
