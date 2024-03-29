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

package disk

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
)

const lastIPFile = "last_reserved_ip"

var defaultDataDir = "/var/lib/cni/networks"

type Store struct {
	*FileLock
	dataDir string
}

func New(network string) (*Store, error) {
	dir := filepath.Join(defaultDataDir, network)
	if err := os.MkdirAll(dir, 0644); err != nil {
		return nil, err
	}

	lk, err := NewFileLock(dir)
	if err != nil {
		return nil, err
	}
	return &Store{lk, dir}, nil
}

func (s *Store) Reserve(id string, ip net.IP) (bool, error) {
	fname := filepath.Join(s.dataDir, ip.String())
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_EXCL|os.O_CREATE, 0644)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(id); err != nil {
		f.Close()
		os.Remove(f.Name())
		return false, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return false, err
	}
	// store the reserved ip in lastIPFile
	ipfile := filepath.Join(s.dataDir, lastIPFile)
	err = ioutil.WriteFile(ipfile, []byte(ip.String()), 0644)
	if err != nil {
		return false, err
	}
	return true, nil
}

// LastReservedIP returns the last reserved IP if exists
func (s *Store) LastReservedIP() (net.IP, error) {
	ipfile := filepath.Join(s.dataDir, lastIPFile)
	data, err := ioutil.ReadFile(ipfile)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve last reserved ip: %v", err)
	}
	return net.ParseIP(string(data)), nil
}

func (s *Store) Release(ip net.IP) error {
	return os.Remove(filepath.Join(s.dataDir, ip.String()))
}

// ReleaseByID N.B. This function eats errors to be tolerant and
// release as much as possible
func (s *Store) ReleaseByID(id string) error {
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if string(data) == id {
			if err := os.Remove(path); err != nil {
				return nil
			}
		}
		return nil
	})
	return err
}

func (s *Store) GetIPByID(id string) (net.IP, error) {
	var ipAddr net.IP
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if string(data) == id {
			ipAddr = net.ParseIP(info.Name())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ipAddr, nil
}
