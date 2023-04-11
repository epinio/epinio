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
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/pkg/errors"
)

func main() {
	runID := os.Getenv("RUN_ID")
	pcp := os.Getenv("RUN_PCP")

	err := DeleteCluster(runID, pcp)
	if err != nil {
		fmt.Println("something failed: " + err.Error())
		os.Exit(1)
	}
}

func DeleteCluster(runID string, pcp string) error {
	switch pcp {
	case "AKS":
		return DeleteClusterAKS(runID)
	case "EKS":
		return DeleteClusterEKS(runID)
	case "GKE":
		return DeleteClusterGKE(runID)
	}

	return nil
}

// Complete cleanup steps for Azure AKS case
func DeleteClusterAKS(runID string) error {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")

	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("AKS_DOMAIN"))
	deletedRecords, err := CleanupDNS(aws_zone_id, domainname)
	if err != nil {
		return errors.Wrap(err, "cleanup DNS failed")
	}

	exists, err := ListClusterAKS(runID)
	if err != nil {
		return errors.Wrap(err, "ListClusterAKS failed")
	}

	// if the cluster didn't exists and we have not deleted any records something was wrong!
	if !exists && len(deletedRecords) == 0 {
		return errors.Wrap(err, "no cluster and no DNS records were deleted. Check your inputs!")
	}

	if exists {
		err := GetKubeconfigAKS(runID)
		if err != nil {
			return errors.Wrap(err, "failed to get kubeconfig")
		}

		err = CleanupNamespaces()
		if err != nil {
			return errors.Wrap(err, "failed to cleanup namespace")
		}

		fmt.Println("Deleting AKS cluster ...")
		out, err := proc.RunW("az", "aks", "delete", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--yes")
		if err != nil {
			return errors.Wrap(err, "failed to delete cluster: "+out)
		}

		fmt.Println("Deleted AKS cluster: ", aks_resource_group+runID)
	}

	return nil
}

// Complete cleanup steps for Amazon EKS case
func DeleteClusterEKS(runID string) error {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	eks_region := os.Getenv("EKS_REGION")

	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("EKS_DOMAIN"))
	deletedRecords, err := CleanupDNS(aws_zone_id, domainname)
	if err != nil {
		return errors.Wrap(err, "cleanup DNS failed")
	}

	exists, err := ListClusterEKS(runID)
	if err != nil {
		return errors.Wrap(err, "ListClusterAKS failed")
	}

	// if the cluster didn't exists and we have not deleted any records something was wrong!
	if !exists && len(deletedRecords) == 0 {
		return errors.Wrap(err, "no cluster and no DNS records were deleted. Check your inputs!")
	}

	if exists {
		err := GetKubeconfigEKS(runID)
		if err != nil {
			return errors.Wrap(err, "failed to get kubeconfig")
		}

		err = CleanupNamespaces()
		if err != nil {
			return errors.Wrap(err, "failed to cleanup namespace")
		}

		fmt.Println("Deleting EKS cluster ...")
		out, err := proc.RunW("eksctl", "delete", "cluster", "--region="+eks_region, "--name=epinio-ci"+runID)
		if err != nil {
			return errors.Wrap(err, "failed to delete cluster: "+out)
		}

		fmt.Println("Deleted EKS cluster: ", "epinio-ci"+runID)
	}

	return nil
}

// Complete cleanup steps for Google GKE case
func DeleteClusterGKE(runID string) error {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	gke_zone := os.Getenv("GKE_ZONE")

	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("GKE_DOMAIN"))
	deletedRecords, err := CleanupDNS(aws_zone_id, domainname)
	if err != nil {
		return errors.Wrap(err, "cleanup DNS failed")
	}

	exists, err := ListClusterGKE(runID)
	if err != nil {
		return errors.Wrap(err, "ListClusterAKS failed")
	}

	// if the cluster didn't exists and we have not deleted any records something was wrong!
	if !exists && len(deletedRecords) == 0 {
		return errors.Wrap(err, "no cluster and no DNS records were deleted. Check your inputs!")
	}

	if exists {
		err := GetKubeconfigGKE(runID)
		if err != nil {
			return errors.Wrap(err, "failed to get kubeconfig")
		}

		err = CleanupNamespaces()
		if err != nil {
			return errors.Wrap(err, "failed to cleanup namespace")
		}

		fmt.Println("Deleting GKE cluster ...")
		os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
		out, err := proc.RunW("gcloud", "container", "clusters", "delete", "epinioci"+runID, "--zone", gke_zone, "--quiet")
		if err != nil {
			return errors.Wrap(err, "failed to delete cluster: "+out)
		}

		fmt.Println("Deleted GKE cluster: ", "epinioci"+runID)
	}

	return nil
}

// Function to verify AWS Route53 DNS records, and clean them up if needed
func CleanupDNS(zoneID string, domainname string) ([]route53.RecordValues, error) {
	deletedRecordValues := []route53.RecordValues{}

	// for each test run, there are two records: domain + wildcard domain
	dnsrecords := [2]string{domainname + ".", "\\052." + domainname + "."}
	fmt.Println("Cleaning up AWS Route53 DNS records ...")

	for _, dnsrecord := range dnsrecords {
		var recordvalues route53.RecordValues
		// We need to get the record by name first - we can only clean up by knowing the correct record type
		recordvalues, err := route53.GetRecord(zoneID, dnsrecord)
		if err != nil {
			return deletedRecordValues, errors.Wrap(err, "get record failed")
		}

		if recordvalues == (route53.RecordValues{}) {
			fmt.Println("AWS Route53 DNS record was cleaned up already, or does not exist: ", dnsrecord)
			continue
		}

		recordname := recordvalues.Name
		recordtype := recordvalues.Type
		recordvalue := recordvalues.Record

		// If the record doesn't exist, we don't need to do anything
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
			return deletedRecordValues, errors.Wrap(err, "updating Route53 failed: "+out)
		}
		fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)

		deletedRecordValues = append(deletedRecordValues, recordvalues)
	}

	return deletedRecordValues, nil
}

// Check if AKS cluster exists
func ListClusterAKS(runID string) (exists bool, err error) {
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	out, err := proc.RunW("az", "aks", "list", "--resource-group", aks_resource_group, "--query", fmt.Sprintf("[].{name:name} | [? contains(name,'%s%s')]", aks_resource_group, runID))
	if err != nil {
		return false, errors.Wrap(err, "azure listing failed")
	}

	if strings.TrimSpace(out) == "[]" {
		fmt.Println("AKS cluster does not exists: " + aks_resource_group + runID)
		return false, nil
	}
	return true, nil
}

// Check if EKS cluster exists
func ListClusterEKS(runID string) (exists bool, err error) {
	eks_region := os.Getenv("EKS_REGION")
	out, err := proc.RunW("aws", "eks", "list-clusters", "--region="+eks_region, "--query", fmt.Sprintf("clusters | [? contains(@,'epinio-ci%s')]", runID), "--output", "text")
	if err != nil {
		return false, errors.Wrap(err, "aws listing failed")
	}

	if strings.TrimSpace(out) == "" {
		fmt.Println("EKS cluster does not exisit: epinio-ci" + runID)
		return false, nil
	}
	return true, nil
}

// Check if GKE cluster exists
func ListClusterGKE(runID string) (exists bool, err error) {
	gke_zone := os.Getenv("GKE_ZONE")
	os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
	out, err := proc.RunW("gcloud", "container", "clusters", "list", "--filter", "epinioci"+runID, "--zone", gke_zone, "--quiet")
	if err != nil {
		return false, errors.Wrap(err, "gke listing failed")
	}

	if strings.TrimSpace(out) == "" {
		fmt.Println("GKE cluster does not exisit: epinioci" + runID)
		return false, nil
	}
	return true, nil
}

func GetKubeconfigAKS(runID string) error {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	out, err := proc.RunW("az", "aks", "get-credentials", "--admin", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--file", kubeconfig_name)
	if err != nil {
		return errors.Wrap(err, "kubeconfig cannot be fetched: "+out)
	}

	fmt.Println("Fetched current kubeconfig")
	return nil
}

func GetKubeconfigEKS(runID string) error {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	eks_region := os.Getenv("EKS_REGION")
	out, err := proc.RunW("eksctl", "utils", "write-kubeconfig", "--region", eks_region, "--cluster", "epinio-ci"+runID, "--kubeconfig", kubeconfig_name)
	if err != nil {
		return errors.Wrap(err, "kubeconfig cannot be fetched: "+out)
	}

	fmt.Println("Fetched current kubeconfig")
	return nil
}

func GetKubeconfigGKE(runID string) error {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	gke_zone := os.Getenv("GKE_ZONE")
	epci_gke_project := os.Getenv("EPCI_GKE_PROJECT")
	os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "true")
	os.Setenv("KUBECONFIG", kubeconfig_name)
	out, err := proc.RunW("gcloud", "container", "clusters", "get-credentials", "epinioci"+runID, "--zone", gke_zone, "--project", epci_gke_project)
	if err != nil {
		return errors.Wrap(err, "kubeconfig cannot be fetched: "+out)
	}

	fmt.Println("Fetched current kubeconfig")
	return nil
}

// Clean up namespaces - therefore unused disks will be removed on cluster deletion
func CleanupNamespaces() error {
	kubeconfig_name := os.Getenv("KUBECONFIG_NAME") + "-Deletion"
	os.Setenv("KUBECONFIG", kubeconfig_name)

	out, err := proc.RunW("kubectl", "--kubeconfig", kubeconfig_name, "delete", "--force", "--ignore-not-found", "namespace", "epinio", "workspace")
	if err != nil {
		return errors.Wrap(err, "failed to cleanup namespace: "+out)
	}

	fmt.Println("Cleaning up test namespaces ...")
	return nil
}
