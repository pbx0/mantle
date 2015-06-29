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

package etcd

import (
	"fmt"

	"github.com/coreos/mantle/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/mantle/platform"
)

func RollingUpgrade(cluster platform.TestCluster) error {
	csize := len(cluster.Machines())

	if plog.LevelAt(capnslog.DEBUG) {
		// get journalctl -f from all machines before starting
		for _, m := range cluster.Machines() {
			if err := m.StartJournal(); err != nil {
				return fmt.Errorf("failed to start journal: %v", err)
			}
		}
	}

	// drop in etcd2.0.12 to /home/core
	plog.Debug("adding files to cluster")
	if err := cluster.DropFile("./kola/tests/etcd/etcd-2.0.12"); err != nil {
		return err
	}

	// drop in etcd2.1-alpha
	if err := cluster.DropFile("./kola/tests/etcd/etcd-2.1.0-alpha.1"); err != nil {
		return err
	}

	// drop in etcd2.1-alpha
	if err := cluster.DropFile("./kola/tests/etcd/etcdctl-2.1.0-alpha.1"); err != nil {
		return err
	}
	// replace existing etcd2 binary with 2.0.12
	plog.Info("replacing etcd with 2.0.12")
	const etcd2 = "/home/core/etcd-2.0.12"
	for _, m := range cluster.Machines() {
		if err := replaceEtcd2Bin(m, etcd2); err != nil {
			return err
		}
	}
	session, err := cluster.Machines()[0].SSHSession()
	if err != nil {
		return err
	}
	b, err := session.CombinedOutput("ls -la")
	if err != nil {
		return err
	}
	plog.Debug(string(b))
	session.Close()
	session, _ = cluster.Machines()[1].SSHSession()
	b, err = session.CombinedOutput("sha1sum < ./etcd-2.0.12")
	if err != nil {
		return err
	}
	plog.Debug(string(b))
	session.Close()

	session, _ = cluster.Machines()[1].SSHSession()
	b, err = session.CombinedOutput("sha1sum ./etcd-2.1.0-alpha.1")
	if err != nil {
		return err
	}
	plog.Debug(string(b))
	session.Close()

	// start 2.0 cluster
	plog.Info("starting 2.0 cluster")
	for _, m := range cluster.Machines() {
		if err := startEtcd2(m); err != nil {
			return err
		}
	}
	for _, m := range cluster.Machines() {
		if err := getClusterHealth(m, csize); err != nil {
			return err
		}
	}

	// rolling replacement
	plog.Info("rolling upgrade to 2.1")
	const etcd21 = "/home/core/etcd-2.1.0-alpha.1"
	for _, m := range cluster.Machines() {
		if err := stopEtcd2(m); err != nil {
			return err
		}

		if err := replaceEtcd2Bin(m, etcd21); err != nil {
			return err
		}

		if err := startEtcd2(m); err != nil {
			return err
		}
	}
	// check cluster health
	for _, m := range cluster.Machines() {
		if err := getClusterHealth(m, csize); err != nil {
			return err
		}
	}
	// check version is now 2.1

	return nil

}
