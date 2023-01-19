// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"log"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Checker is the interface that wraps the Check method that checks if cfg can be used to connect to
// the Kubernetes cluster.
type Checker interface {
	Check(cfg *rest.Config) error
}

// NewChecker constructs a default checker that satisfies the Checker interface.
func NewChecker() Checker {
	return &checker{
		createClientSet:    kubernetes.NewForConfig,
		checkServerVersion: checkServerVersion,
	}
}

type checker struct {
	createClientSet    func(c *rest.Config) (*kubernetes.Clientset, error)
	checkServerVersion func(d discovery.ServerVersionInterface) error
}

func (c *checker) Check(cfg *rest.Config) error {
	clientset, err := c.createClientSet(cfg)
	if err != nil {
		return &checkConfigError{err}
	}
	err = c.checkServerVersion(clientset.Discovery())
	if err != nil {
		return &checkConfigError{err}
	}
	return nil
}

type checkConfigError struct {
	err error
}

func (e *checkConfigError) Error() string {
	return fmt.Sprintf("invalid kube config: %v", e.err)
}

func checkServerVersion(d discovery.ServerVersionInterface) error {
	_, err := d.ServerVersion()
	return err
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
