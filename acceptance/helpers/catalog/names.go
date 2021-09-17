// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

func NewTmpName(base string) string {
	return base + strconv.Itoa(int(time.Now().Nanosecond()))
}

func NewOrgName() string {
	return "namespace-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func NewAppName() string {
	return "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func NewServiceName() string {
	return "service-" + strconv.Itoa(int(time.Now().Nanosecond())) + strconv.Itoa(rand.Int()) // nolint:gosec // Non-crypto use
}

func GetServiceBindingName(orgName, serviceName, appName string) string {
	return fmt.Sprintf("svc.org-%s.svc-%s.app-%s", orgName, serviceName, appName)
}
