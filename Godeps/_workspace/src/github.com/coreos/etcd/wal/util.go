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

package wal

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/coreos/mantle/Godeps/_workspace/src/github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/mantle/Godeps/_workspace/src/github.com/coreos/etcd/pkg/types"
)

// WalVersion is an enum for versions of etcd logs.
type WalVersion string

const (
	WALUnknown  WalVersion = "Unknown WAL"
	WALNotExist WalVersion = "No WAL"
	WALv0_4     WalVersion = "0.4.x"
	WALv2_0     WalVersion = "2.0.0"
	WALv2_0_1   WalVersion = "2.0.1"
)

func DetectVersion(dirpath string) (WalVersion, error) {
	names, err := fileutil.ReadDir(dirpath)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		// Error reading the directory
		return WALNotExist, err
	}
	if len(names) == 0 {
		// Empty WAL directory
		return WALNotExist, nil
	}
	nameSet := types.NewUnsafeSet(names...)
	if nameSet.Contains("member") {
		ver, err := DetectVersion(path.Join(dirpath, "member"))
		if ver == WALv2_0 {
			return WALv2_0_1, nil
		} else if ver == WALv0_4 {
			// How in the blazes did it get there?
			return WALUnknown, nil
		}
		return ver, err
	}
	if nameSet.ContainsAll([]string{"snap", "wal"}) {
		// .../wal cannot be empty to exist.
		if Exist(path.Join(dirpath, "wal")) {
			return WALv2_0, nil
		}
	}
	if nameSet.ContainsAll([]string{"snapshot", "conf", "log"}) {
		return WALv0_4, nil
	}

	return WALUnknown, nil
}

func Exist(dirpath string) bool {
	names, err := fileutil.ReadDir(dirpath)
	if err != nil {
		return false
	}
	return len(names) != 0
}

// searchIndex returns the last array index of names whose raft index section is
// equal to or smaller than the given index.
// The given names MUST be sorted.
func searchIndex(names []string, index uint64) (int, bool) {
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		_, curIndex, err := parseWalName(name)
		if err != nil {
			log.Panicf("parse correct name should never fail: %v", err)
		}
		if index >= curIndex {
			return i, true
		}
	}
	return -1, false
}

// names should have been sorted based on sequence number.
// isValidSeq checks whether seq increases continuously.
func isValidSeq(names []string) bool {
	var lastSeq uint64
	for _, name := range names {
		curSeq, _, err := parseWalName(name)
		if err != nil {
			log.Panicf("parse correct name should never fail: %v", err)
		}
		if lastSeq != 0 && lastSeq != curSeq-1 {
			return false
		}
		lastSeq = curSeq
	}
	return true
}

func checkWalNames(names []string) []string {
	wnames := make([]string, 0)
	for _, name := range names {
		if _, _, err := parseWalName(name); err != nil {
			log.Printf("wal: parse %s error: %v", name, err)
			continue
		}
		wnames = append(wnames, name)
	}
	return wnames
}

func parseWalName(str string) (seq, index uint64, err error) {
	_, err = fmt.Sscanf(str, "%016x-%016x.wal", &seq, &index)
	return
}

func walName(seq, index uint64) string {
	return fmt.Sprintf("%016x-%016x.wal", seq, index)
}
