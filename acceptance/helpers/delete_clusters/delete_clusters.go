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

package main

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
)

func main() {
	runID, runIDcheck := os.LookupEnv("RUN_ID")
	pcp, pcpcheck := os.LookupEnv("RUN_PCP")
	if !runIDcheck || !pcpcheck {
		if !runIDcheck {
			fmt.Println("Error: Variable RUN_ID not set in env")
		}
		if !pcpcheck {
			fmt.Println("Error: Variable RUN_PCP not set in env")
		}
		return
	}

	DeleteCluster(runID, pcp)
}

func CleanupDNS(zoneID string, domainname string) {
	dnsrecords := [2]string{domainname + ".", "\\052." + domainname + "."}
	fmt.Println("Cleaning up AWS Route53 DNS records ...")

	for _, dnsrecord := range dnsrecords {
		Name, Type, Record, err := route53.GetRecord(zoneID, dnsrecord)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		if Name != "Clean" {
			var change route53.ChangeResourceRecordSet
			switch Type {
			case "A":
				change = route53.A(Name, Record, "DELETE")
			case "CNAME":
				change = route53.CNAME(Name, Record, "DELETE")
			}
			out, err := route53.Update(zoneID, change, "")
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)
			}
		} else {
			fmt.Println("AWS Route53 DNS record was cleaned up already, or does not exist: ", dnsrecord)
		}
	}

}

func ListCluster(runID string, pcp string) (exists bool) {
	switch pcp {
	case "AKS":
		aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
		out, err := proc.RunW("az", "aks", "list", "--resource-group", aks_resource_group, "--query", "[].{name:name} | [? contains(name,'"+aks_resource_group+runID+"')]")
		if err != nil {
			fmt.Println("Error: ", err, out)
			exists = false
		} else {
			if out == "[]" || out == "[]\n" {
				fmt.Println("AKS cluster does not exisit: " + aks_resource_group + runID)
				exists = false
			} else {
				exists = true
			}
		}
	case "EKS":
		eks_region := os.Getenv("EKS_REGION")
		out, err := proc.RunW("aws", "eks", "list-clusters", "--region="+eks_region, "--query", "clusters | [? contains(@,'epinio-ci"+runID+"')]", "--output", "text")
		if err != nil {
			fmt.Println("Error: ", err, out)
			exists = false
		} else {
			if out == "" {
				fmt.Println("EKS cluster does not exisit: epinio-ci" + runID)
				exists = false
			} else {
				exists = true
			}
		}
	case "GKE":
		gke_zone := os.Getenv("GKE_ZONE")
		os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
		out, err := proc.RunW("gcloud", "container", "clusters", "list", "--filter", "epinioci"+runID, "--zone", gke_zone, "--quiet")
		if err != nil {
			fmt.Println("Error: ", err, out)
			exists = false
		} else {
			if out == "" {
				fmt.Println("GKE cluster does not exisit: epinioci" + runID)
				exists = false
			} else {
				exists = true
			}
		}
	}
	return exists
}

func GetKubeconfig(runID string, pcp string) {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME")+"-Deletion"

	switch pcp {
		case "AKS":
			aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
			out, err := proc.RunW("az", "aks", "get-credentials", "--admin", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--file", kubeconfig_name)
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Fetched current kubeconfig")
			}
		case "EKS":
			eks_region := os.Getenv("EKS_REGION")
			out, err := proc.RunW("eksctl", "utils", "write-kubeconfig", "--region", eks_region, "--cluster", "epinio-ci"+runID, "--kubeconfig", kubeconfig_name)
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Fetched current kubeconfig")
			}
		case "GKE":
			gke_zone := os.Getenv("GKE_ZONE")
			epci_gke_project := os.Getenv("EPCI_GKE_PROJECT")
			os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
			os.Setenv("KUBECONFIG", kubeconfig_name)
			out, err := proc.RunW("gcloud", "container", "clusters", "get-credentials", "epinioci"+runID, "--zone", gke_zone, "--project", epci_gke_project)
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Fetched current kubeconfig")
			}
		}
}

func CleanupNamespaces() {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME")+"-Deletion"
	os.Setenv("KUBECONFIG", kubeconfig_name)

	out, err := proc.RunW("kubectl", "--kubeconfig", kubeconfig_name, "delete", "--force", "--ignore-not-found", "namespace", "epinio", "workspace")
	if err != nil {
		fmt.Println("Error: ", err, out)
	} else {
		fmt.Println("Cleaning up test namespaces ...")
	}
}

func DeleteCluster(runID string, pcp string) {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")

	switch pcp {
	case "AKS":
		aks_domain := os.Getenv("AKS_DOMAIN")
		aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
		domainname := "id" + runID + "-" + aks_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists {
			GetKubeconfig(runID, pcp)
			CleanupNamespaces()
			fmt.Println("Deleting AKS cluster ...")
			out, err := proc.RunW("az", "aks", "delete", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--yes")
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted AKS cluster: ", aks_resource_group+runID)
			}
		}
	case "EKS":
		eks_domain := os.Getenv("EKS_DOMAIN")
		eks_region := os.Getenv("EKS_REGION")
		domainname := "id" + runID + "-" + eks_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists {
			GetKubeconfig(runID, pcp)
			CleanupNamespaces()
			fmt.Println("Deleting EKS cluster ...")
			out, err := proc.RunW("eksctl", "delete", "cluster", "--region="+eks_region, "--name=epinio-ci"+runID)
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted EKS cluster: ", "epinio-ci"+runID)
			}
		}
	case "GKE":
		gke_domain := os.Getenv("GKE_DOMAIN")
		gke_zone := os.Getenv("GKE_ZONE")
		domainname := "id" + runID + "-" + gke_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists {
			GetKubeconfig(runID, pcp)
			CleanupNamespaces()
			fmt.Println("Deleting GKE cluster ...")
			os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
			out, err := proc.RunW("gcloud", "container", "clusters", "delete", "epinioci"+runID, "--zone", gke_zone, "--quiet")
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted GKE cluster: ", "epinioci"+runID)
			}
		}
	}
}
