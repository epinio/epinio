// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
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
