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

package routes_test

import (
	. "github.com/epinio/epinio/internal/routes"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Route", func() {
	Describe("FromString", func() {
		var routeStr string
		BeforeEach(func() {
			routeStr = "mydomain.org/api/v1"
		})
		It("constructs a Route object", func() {
			Expect(FromString(routeStr)).To(Equal(Route{
				Domain: "mydomain.org",
				Path:   "/api/v1",
			}))
		})
		When("a path doesn't exist", func() {
			BeforeEach(func() {
				routeStr = "mydomain.org"
			})

			It("constructs a Route object with path set to \"/\"", func() {
				Expect(FromString(routeStr)).To(Equal(Route{
					Domain: "mydomain.org",
					Path:   "/",
				}))
			})
		})
	})

	Describe("FromIngress", func() {
		var routeIngress networkingv1.Ingress
		BeforeEach(func() {
			routeIngress = networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testIngress",
				},
				Spec: networkingv1.IngressSpec{
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "someconfiguration",
							Port: networkingv1.ServiceBackendPort{
								Name:   "http",
								Number: 80,
							},
						},
					},
					TLS: []networkingv1.IngressTLS{},
					Rules: []networkingv1.IngressRule{
						{
							Host: "mydomain.org",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/api/v1",
										}}}}}}}}
		})
		It("returns a list of Route objects", func() {
			result, err := FromIngress(routeIngress)
			Expect(err).ToNot(HaveOccurred())
			Expect(result[0]).To(Equal(Route{Domain: "mydomain.org", Path: "/api/v1"}))
			Expect(len(result)).To(Equal(1))
		})
		When("the Ingress has no rules defined", func() {
			BeforeEach(func() {
				routeIngress.Spec.Rules = []networkingv1.IngressRule{}
			})
			It("returns an error", func() {
				_, err := FromIngress(routeIngress)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("no Rules found on Ingress"))
			})
		})
		When("the Ingress has multiple rules defined", func() {
			BeforeEach(func() {
				routeIngress.Spec.Rules = append(routeIngress.Spec.Rules, networkingv1.IngressRule{
					Host: "someotherdomain.org",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/otherapi/v1",
								}}}}})
			})
			It("creates Routes out of every rule", func() {
				result, err := FromIngress(routeIngress)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal([]Route{
					{
						Domain: "mydomain.org",
						Path:   "/api/v1",
					},
					{
						Domain: "someotherdomain.org",
						Path:   "/otherapi/v1",
					},
				}))
			})
		})
	})

	Describe("ToIngress", func() {
		var route Route
		BeforeEach(func() {
			route = Route{
				Domain: "somedomain.org",
				Path:   "/api/v1",
			}
		})
		It("creates an Ingress matching the Route", func() {
			ingress := route.ToIngress("myingress")
			Expect(ingress.ObjectMeta.Name).To(Equal("myingress"))
			Expect(len(ingress.Spec.Rules)).To(Equal(1))
			Expect(ingress.Spec.Rules[0].Host).To(Equal("somedomain.org"))
			Expect(len(ingress.Spec.Rules[0].HTTP.Paths)).To(Equal(1))
			Expect(ingress.Spec.Rules[0].HTTP.Paths[0].Path).To(Equal("/api/v1"))
		})
	})
	Describe("String", func() {
		var route Route
		BeforeEach(func() {
			route = Route{
				Domain: "somedomain.org",
				Path:   "/somepath",
			}
		})
		It("returns the string representation of the Route", func() {
			Expect(route.String()).To(Equal("somedomain.org/somepath"))
		})
		When("the path is empty", func() {
			BeforeEach(func() {
				route.Path = ""
			})
			It("doesn't add a \"/\" at the end of the string", func() {
				Expect(route.String()).To(Equal("somedomain.org"))
			})
		})
		When("the path is \"/\"", func() {
			BeforeEach(func() {
				route.Path = "/"
			})
			It("doesn't add a \"/\" at the end of the string", func() {
				Expect(route.String()).To(Equal("somedomain.org"))
			})
		})
	})
})
