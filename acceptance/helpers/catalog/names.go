// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
	"fmt"
	"strconv"
	"time"
)

func NewOrgName() string {
	return "orgs-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func NewAppName() string {
	return "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func NewServiceName() string {
	return "service-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func GetServiceBindingName(orgName, serviceName, appName string) string {
	return fmt.Sprintf("svc.org-%s.svc-%s.app-%s", orgName, serviceName, appName)
}
