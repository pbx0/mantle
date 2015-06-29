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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/mantle/platform"
)

// run etcd on each cluster machine
func startEtcd2(m platform.Machine) error {
	etcdStart := "sudo systemctl start etcd2.service"
	_, err := m.SSH(etcdStart)
	if err != nil {
		return fmt.Errorf("start etcd2.service on %v failed: %s", m.IP(), err)
	}
	return nil
}

// stop etcd on each cluster machine
func stopEtcd2(m platform.Machine) error {
	// start etcd instance
	etcdStop := "sudo systemctl stop etcd2.service"
	_, err := m.SSH(etcdStop)
	if err != nil {
		return fmt.Errorf("stop etcd.2service on failed: %s", err)
	}
	return nil
}

type Key struct {
	Node struct {
		Value string `json:"value"`
	} `json:"node"`
}

func setKey(cluster platform.Cluster, key, value string) error {
	cmd := cluster.NewCommand("curl", "-L", "http://10.0.0.3:4001/v2/keys/"+key, "-XPUT", "-d", "value="+value)
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error setting value to cluster: %v", err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", b)

	var k Key
	err = json.Unmarshal(b, &k)
	if err != nil {
		return err
	}
	if k.Node.Value != value {
		fmt.Errorf("etcd key not set correctly")
	}
	return nil
}

func getKey(cluster platform.Cluster, key string) (string, error) {
	cmd := cluster.NewCommand("curl", "-L", "http://10.0.0.3:4001/v2/keys/"+key)
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting old value from cluster: %v", err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", b)

	var k Key
	err = json.Unmarshal(b, &k)
	if err != nil {
		return "", err
	}

	return k.Node.Value, nil
}

// replace default binary for etcd2.service with given binary
func replaceEtcd2Bin(m platform.Machine, newPath string) error {
	if !filepath.IsAbs(newPath) {
		return fmt.Errorf("newPath must be an absolute filepath")
	}

	override := "\"[Service]\nExecStart=\nExecStart=" + newPath + "\""
	_, err := m.SSH(fmt.Sprintf("echo %v | sudo tee /run/systemd/system/etcd2.service.d/99-exec.conf", override))
	if err != nil {
		return err
	}
	_, err = m.SSH("sudo systemctl daemon-reload")
	if err != nil {
		return err
	}
	return nil
}

func getEtcdInternalVersion(cluster platform.Cluster) (int, error) {
	cmd := cluster.NewCommand("curl", "-L", "http://10.0.0.2:4001/version")
	b, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error curling version: %v", err)
	}

	type Version struct {
		Internal string `json:"internalVersion"`
	}
	var v Version

	err = json.Unmarshal(b, &v)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(v.Internal)
}

// poll cluster-health until result
func getClusterHealth(m platform.Machine, csize int) error {
	const (
		retries   = 5
		retryWait = 3 * time.Second
	)
	var err error
	var b []byte

	for i := 0; i < retries; i++ {
		plog.Info("polling cluster health...")
		b, err = m.SSH("./etcdctl-2.1.0-alpha.1 cluster-health")
		if err != nil {
			plog.Infof("sleeping, hit failure %v", err)
			plog.Debug(string(b))
			time.Sleep(retryWait)
			continue
		}
		plog.Debug(string(b))

		// repsonse should include "healthy" for each machine and for cluster
		if strings.Count(string(b), "healthy") == csize+1 {
			plog.Debug(string(b))
			return nil
		}
	}

	if err != nil {
		return fmt.Errorf("health polling failed: %v: %s", err, b)
	} else {
		return fmt.Errorf("status unhealthy or incomplete: %s", b)
	}
}
