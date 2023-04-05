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

// delete_clusters.go called from acceptance test workflows and delete_clusters.yml

package main

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
)

var noaction []int

func main() {
	runID := os.Getenv("RUN_ID")
	pcp := os.Getenv("RUN_PCP")

	DeleteCluster(runID, pcp)
}

// Function to verify AWS Route53 DNS records, and clean them up if needed
func CleanupDNS(zoneID string, domainname string) {
	// for each test run, there are two records: domain + wildcard domain
	dnsrecords := [2]string{domainname + ".", "\\052." + domainname + "."}
	fmt.Println("Cleaning up AWS Route53 DNS records ...")

	for _, dnsrecord := range dnsrecords {
		var recordvalues route53.RecordValues
		// We need to get the record by name first - we can only clean up by knowing the correct record type
		recordvalues, err := route53.GetRecord(zoneID, dnsrecord)
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}
		recordname := recordvalues.Name
		recordtype := recordvalues.Type
		recordvalue := recordvalues.Record
		// If the record doesn't exist, we don't need to do anything
		if recordname == "Clean" {
			// Append to noaction array for each record that didn't exist
			noaction = append(noaction, 1)
			fmt.Println("AWS Route53 DNS record was cleaned up already, or does not exist: ", dnsrecord)
			continue
		}
		var change route53.ChangeResourceRecordSet
		switch recordtype {
		case "A":
			change = route53.A(recordname, recordvalue, "DELETE")
		case "CNAME":
			change = route53.CNAME(recordname, recordvalue, "DELETE")
		}
		// Delete existing DNS record
		out, err := route53.Update(zoneID, change, "")
		if err != nil {
			fmt.Println("Error: ", err, out)
			os.Exit(1)
		} else {
			fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)
		}
	}

}

// Check if AKS cluster exists
func ListClusterAKS(runID string) (exists bool) {
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	out, err := proc.RunW("az", "aks", "list", "--resource-group", aks_resource_group, "--query", fmt.Sprintf("[].{name:name} | [? contains(name,'%s%s')]", aks_resource_group, runID))
	if err != nil {
		fmt.Println("Error: ", err, out)
		os.Exit(1)
	} else {
		if out == "[]" || out == "[]\n" {
			fmt.Println("AKS cluster does not exisit: " + aks_resource_group + runID)
			exists = false
			// If no DNS record was cleaned up, we will exit here - wrong runID?
			if len(noaction) > 1 {
				fmt.Println("Error: Nothing was cleaned up. Please check your inputs.")
				os.Exit(1)
			}
		} else {
			exists = true
		}
	}
	return exists
}

// Check if EKS cluster exists
func ListClusterEKS(runID string) (exists bool) {
	eks_region := os.Getenv("EKS_REGION")
	out, err := proc.RunW("aws", "eks", "list-clusters", "--region="+eks_region, "--query", fmt.Sprintf("clusters | [? contains(@,'epinio-ci%s')]", runID), "--output", "text")
	if err != nil {
		fmt.Println("Error: ", err, out)
		os.Exit(1)
	} else {
		if out == "" {
			fmt.Println("EKS cluster does not exisit: epinio-ci" + runID)
			exists = false
			// If no DNS record was cleaned up, we will exit here - wrong runID?
			if len(noaction) > 1 {
				fmt.Println("Error: Nothing was cleaned up. Please check your inputs.")
				os.Exit(1)
			}
		} else {
			exists = true
		}
	}
	return exists
}

// Check if GKE cluster exists
func ListClusterGKE(runID string) (exists bool) {
	gke_zone := os.Getenv("GKE_ZONE")
	os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
	out, err := proc.RunW("gcloud", "container", "clusters", "list", "--filter", "epinioci"+runID, "--zone", gke_zone, "--quiet")
	if err != nil {
		fmt.Println("Error: ", err, out)
		os.Exit(1)
	} else {
		if out == "" {
			fmt.Println("GKE cluster does not exisit: epinioci" + runID)
			exists = false
			// If no DNS record was cleaned up, we will exit here - wrong runID?
			if len(noaction) > 1 {
				fmt.Println("Error: Nothing was cleaned up. Please check your inputs.")
				os.Exit(1)
			}
		} else {
			exists = true
		}
	}
	return exists
}

func GetKubeconfigAKS(runID string) {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	out, err := proc.RunW("az", "aks", "get-credentials", "--admin", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--file", kubeconfig_name)
	if err != nil {
		fmt.Println("Error: ", err, out)
	} else {
		fmt.Println("Fetched current kubeconfig")
	}
}

func GetKubeconfigEKS(runID string) {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	eks_region := os.Getenv("EKS_REGION")
	out, err := proc.RunW("eksctl", "utils", "write-kubeconfig", "--region", eks_region, "--cluster", "epinio-ci"+runID, "--kubeconfig", kubeconfig_name)
	if err != nil {
		fmt.Println("Error: ", err, out)
	} else {
		fmt.Println("Fetched current kubeconfig")
	}
}

func GetKubeconfigGKE(runID string) {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
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

// Clean up namespaces - therefore unused disks will be removed on cluster deletion
func CleanupNamespaces() {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	os.Setenv("KUBECONFIG", kubeconfig_name)

	out, err := proc.RunW("kubectl", "--kubeconfig", kubeconfig_name, "delete", "--force", "--ignore-not-found", "namespace", "epinio", "workspace")
	if err != nil {
		fmt.Println("Error: ", err, out)
		os.Exit(1)
	} else {
		fmt.Println("Cleaning up test namespaces ...")
	}
}

// Complete cleanup steps for Azure AKS case
func DeleteClusterAKS(runID string) {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("AKS_DOMAIN"))
	CleanupDNS(aws_zone_id, domainname)
	exists := ListClusterAKS(runID)
	if exists {
		GetKubeconfigAKS(runID)
		CleanupNamespaces()
		fmt.Println("Deleting AKS cluster ...")
		out, err := proc.RunW("az", "aks", "delete", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--yes")
		if err != nil {
			fmt.Println("Error: ", err, out)
			os.Exit(1)
		} else {
			fmt.Println("Deleted AKS cluster: ", aks_resource_group+runID)
		}
	}
}

// Complete cleanup steps for Amazon EKS case
func DeleteClusterEKS(runID string) {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	eks_region := os.Getenv("EKS_REGION")
	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("EKS_DOMAIN"))
	CleanupDNS(aws_zone_id, domainname)
	exists := ListClusterEKS(runID)
	if exists {
		GetKubeconfigEKS(runID)
		CleanupNamespaces()
		fmt.Println("Deleting EKS cluster ...")
		out, err := proc.RunW("eksctl", "delete", "cluster", "--region="+eks_region, "--name=epinio-ci"+runID)
		if err != nil {
			fmt.Println("Error: ", err, out)
			os.Exit(1)
		} else {
			fmt.Println("Deleted EKS cluster: ", "epinio-ci"+runID)
		}
	}
}

// Complete cleanup steps for Google GKE case
func DeleteClusterGKE(runID string) {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	gke_zone := os.Getenv("GKE_ZONE")
	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("GKE_DOMAIN"))
	CleanupDNS(aws_zone_id, domainname)
	exists := ListClusterGKE(runID)
	if exists {
		GetKubeconfigGKE(runID)
		CleanupNamespaces()
		fmt.Println("Deleting GKE cluster ...")
		os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
		out, err := proc.RunW("gcloud", "container", "clusters", "delete", "epinioci"+runID, "--zone", gke_zone, "--quiet")
		if err != nil {
			fmt.Println("Error: ", err, out)
			os.Exit(1)
		} else {
			fmt.Println("Deleted GKE cluster: ", "epinioci"+runID)
		}
	}
}

func DeleteCluster(runID string, pcp string) {
	switch pcp {
	case "AKS":
		DeleteClusterAKS(runID)
	case "EKS":
		DeleteClusterEKS(runID)
	case "GKE":
		DeleteClusterGKE(runID)
	}
}
