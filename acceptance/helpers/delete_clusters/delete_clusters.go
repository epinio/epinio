// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
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
		fmt.Println("Error: " + err.Error())
		os.Exit(1)
	}
}

func DeleteCluster(runID string, pcp string) error {
	switch pcp {
	case "AWS_RKE2":
		return DeleteClusterAWS_RKE2(runID)
	}

	return nil
}

// Complete cleanup steps for AWS-EC2 RKE2 case
func DeleteClusterAWS_RKE2(runID string) error {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")

	domainname := fmt.Sprintf("id%s-%s", runID, os.Getenv("AWS_RKE2_DOMAIN"))
	deletedRecords, err := CleanupDNS(aws_zone_id, domainname)
	if err != nil {
		return errors.Wrap(err, "CleanupDNS failed")
	}

	exists, err := ListInstancesAWS_RKE2(runID)
	if err != nil {
		return errors.Wrap(err, "ListInstancesAWS_RKE2 failed")
	}

	// if the instances didn't exist and we have not deleted any records something was wrong!
	if !exists && len(deletedRecords) == 0 {
		return errors.New("Nothing was cleaned up. Please check your input values!")
	}

	if exists {
		if os.Getenv("FETCH_KUBECONFIG") == "true" {
			err := GetKubeconfigAWS_RKE2(runID)
			if err != nil {
				return errors.Wrap(err, "GetKubeconfigAWS_RKE2 failed")
			}
		}

		err = CleanupAWS_RKE2()
		if err != nil {
			return errors.Wrap(err, "CleanupAWS_RKE2 failed")
		}

		fmt.Println("Deleting EC2 instances ...")
		out, err := proc.RunW("aws", "ec2", "describe-instances", "--filters", fmt.Sprintf("Name=tag:Name,Values='epinio-rke2-ci%s'", runID), "--query", "Reservations[*].Instances[*].InstanceId", "--output", "text")
		if err != nil {
			return errors.Wrap(err, "aws cli command failed: "+out)
		}
		instance_ids := strings.TrimSpace(out)

		out, err = proc.RunW("aws", "ec2", "terminate-instances", "--instance-ids", instance_ids)
		if err != nil {
			return errors.Wrap(err, "Failed to delete instances: "+out)
		}
		fmt.Println("Deleted EC2 instances: " + instance_ids)
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
			return deletedRecordValues, errors.Wrap(err, "GetRecord failed")
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
			return deletedRecordValues, errors.Wrap(err, "Update AWS Route53 failed: "+out)
		}
		fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)

		deletedRecordValues = append(deletedRecordValues, recordvalues)
	}

	return deletedRecordValues, nil
}

// Check if EC2 AWS_RKE2 instances exist
func ListInstancesAWS_RKE2(runID string) (exists bool, err error) {
	out, err := proc.RunW("aws", "ec2", "describe-instances", "--query", fmt.Sprintf("Reservations[].Instances[].Tags[].{Name:Value} | [? contains(Name,'epinio-rke2-ci%s')]", runID), "--output", "text")
	if err != nil {
		return false, errors.Wrap(err, "aws cli command failed: "+out)
	}

	if strings.TrimSpace(out) == "" {
		fmt.Println("EC2 instances do not exist: epinio-rke2-ci" + runID)
		return false, nil
	}

	return true, nil
}

func GetKubeconfigAWS_RKE2(runID string) error {
	kubeconfig := os.Getenv("KUBECONFIG")

	// Note: kubeconfig path comes from a controlled test environment variable in CI.
	// It is not user-supplied input and is only used in acceptance tests.
	// #nosec G304 - path is trusted in this testing helper
	out, err := proc.RunW("aws", "ec2", "describe-instances", "--filters", fmt.Sprintf("Name=tag:Name,Values='epinio-rke2-ci%s'", runID), "--query", "Reservations[*].Instances[*].PublicDnsName", "--output", "text")
	if err != nil {
		return errors.Wrap(err, "aws cli command failed: "+out)
	}

	server_hostname := strings.TrimSpace(out)
	server_config, err := proc.RunW("ssh", "-o", "BatchMode=yes", "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", "-o", "LogLevel=error", "-o", "ConnectTimeout=30", "-o", "User=ec2-user", "-i", "~/.ssh/id_rsa_ec2.pem", server_hostname, "cat /etc/rancher/rke2/rke2.yaml")
	if err != nil {
		return errors.Wrap(err, "Failed to get /etc/rancher/rke2/rke2.yaml "+server_config)
	}

	kubeconfig_rke2 := []byte(strings.Replace(server_config, "127.0.0.1", server_hostname, 1))
	err = os.WriteFile(kubeconfig, kubeconfig_rke2, 0600) // nolint:gosec // kubeconfig path from acceptance test setup
	if err != nil {
		return errors.Wrap(err, "Failed to create "+kubeconfig)
	}

	fmt.Println("Fetched current kubeconfig")
	return nil
}

// Clean up namespaces and resources from AWS_RKE2 setup
func CleanupAWS_RKE2() error {
	kubeconfig := os.Getenv("KUBECONFIG")

	fmt.Println("Cleaning up ingress-nginx and namespaces ...")
	out, err := proc.RunW("helm", "--kubeconfig", kubeconfig, "delete", "ingress-nginx", "-n", "ingress-nginx", "--wait")
	if err != nil {
		return errors.Wrap(err, "helm cli command failed: "+out)
	}

	out, err = proc.RunW("kubectl", "--kubeconfig", kubeconfig, "delete", "--force", "--ignore-not-found", "namespace", "epinio", "workspace", "ingress-nginx")
	if err != nil {
		return errors.Wrap(err, "kubectl cli command failed: "+out)
	}

	return nil
}
