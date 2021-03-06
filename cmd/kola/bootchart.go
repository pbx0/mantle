// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/coreos/mantle/cli"
	"github.com/coreos/mantle/platform"
)

var cmdBootchart = &cli.Command{
	Run:     runBootchart,
	Name:    "bootchart",
	Summary: "Boot performance graphing tool",
	Usage:   "> bootchart.svg",
	Description: `
Boot a single instance and plot how the time was spent.

Note that this actually uses systemd-analyze plot rather than
systemd-bootchart since the latter requires setting a different
init process.

This must run as root!
`}

func init() {
	cli.Register(cmdBootchart)
}

func runBootchart(args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "No args accepted\n")
		return 2
	}

	cluster, err := platform.NewQemuCluster()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cluster failed: %v\n", err)
		return 1
	}
	defer cluster.Destroy()

	m, err := cluster.NewMachine("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Machine failed: %v\n", err)
		return 1
	}
	defer m.Destroy()

	ssh, err := m.SSHSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH failed: %v\n", err)
		return 1
	}

	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	if err = ssh.Run("systemd-analyze plot"); err != nil {
		fmt.Fprintf(os.Stderr, "SSH failed: %v\n", err)
		return 1
	}

	return 0
}
