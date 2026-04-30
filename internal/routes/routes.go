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

package routes

import (
	"errors"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Route struct {
	Domain string
	Path   string
}

// String returns the string representation of a Route object.
// E.g.
// Route{ Domain: "mydomain.org", Path: "/api" }
// becomes: "mydomain.org/api"
// also removes trailing "/". E.g.
// Route{ Domain: "mydomain.org", Path: "/" }
// becomes: "mydomain.org" (no trailing "/")
func (r Route) String() string {
	return strings.TrimSuffix(r.Domain+r.Path, "/")
}

// ToIngress  returns an Ingress resource for this route
func (r Route) ToIngress(ingressName string) networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific

	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: r.Domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     r.Path,
									PathType: &pathTypeImplementationSpecific,
								}}}}}}}}
}

// FromString converts a route string to a Route object.
// E.g.
// mydomain.org/api
// becomes: Route{ Domain: "mydomain.org", Path: "/api" }
func FromString(routeStr string) Route {
	var domain, path string

	splitRoute := strings.SplitN(routeStr, "/", 2)
	domain = splitRoute[0]
	if len(splitRoute) > 1 {
		path = "/" + splitRoute[1]
	} else {
		path = "/"
	}
	return Route{Domain: domain, Path: path}
}

// FromIngress returns a Route resource matching the given Ingress
// NOTE: Epinio doesn't create Ingresses with multiple rules. For that reason,
// this function will try to construct a Route from the first rule of the passed
// Ingress, ingoring all other rules if they exist.
func FromIngress(ingress networkingv1.Ingress) ([]Route, error) {
	if len(ingress.Spec.Rules) == 0 {
		return nil, errors.New("no Rules found on Ingress")
	}

	result := []Route{}
	for _, r := range ingress.Spec.Rules {
		domain := r.Host
		for _, p := range r.HTTP.Paths {
			result = append(result, Route{Domain: domain, Path: p.Path})
		}
	}

	return result, nil
}
