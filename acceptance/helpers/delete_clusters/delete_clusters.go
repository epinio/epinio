package main

import (
	"fmt"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"os"
)

func main() {
	runID := os.Getenv("RUN_ID")
	pcp := os.Getenv("RUN_PCP")

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
			switch Type {
			case "A":
				change := route53.A(Name, Record, "DELETE")
				out, err := route53.Update(zoneID, change, nodeTmpDir)
				if err != nil {
					fmt.Println("Error: ", err, out)
				} else {
					fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)
				}
			case "CNAME":
				change := route53.CNAME(Name, Record, "DELETE")
				out, err := route53.Update(zoneID, change, nodeTmpDir)
				if err != nil {
					fmt.Println("Error: ", err, out)
				} else {
					fmt.Println("Cleaned up AWS Route53 DNS record: ", dnsrecord)
				}
			}

		} else {
			fmt.Println("AWS Route53 DNS record was cleaned up already, or does not exist: ", dnsrecord)
		}
	}

}

func ListCluster(runID string, pcp string) (exists bool) {
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	eks_region := os.Getenv("EKS_REGION")
	gke_zone := os.Getenv("GKE_ZONE")

	switch pcp {
	case "AKS":
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

func DeleteCluster(runID string, pcp string) {
	aws_zone_id := os.Getenv("AWS_ZONE_ID")
	aks_domain := os.Getenv("AKS_DOMAIN")
	aks_resource_group := os.Getenv("AKS_RESOURCE_GROUP")
	eks_domain := os.Getenv("EKS_DOMAIN")
	eks_region := os.Getenv("EKS_REGION")
	gke_zone := os.Getenv("GKE_ZONE")
	gke_domain := os.Getenv("GKE_DOMAIN")

	switch pcp {
	case "AKS":
		domainname := "id" + runID + "-" + aks_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists == true {
			fmt.Println("Deleting AKS cluster ...")
			out, err := proc.RunW("az", "aks", "delete", "--resource-group", aks_resource_group, "--name", aks_resource_group+runID, "--yes")
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted AKS cluster: ", aks_resource_group+runID)
			}
		}
	case "EKS":
		domainname := "id" + runID + "-" + eks_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists == true {
			fmt.Println("Deleting EKS cluster ...")
			out, err := proc.RunW("eksctl", "delete", "cluster", "--region="+eks_region, "--name=epinio-ci"+runID)
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted EKS cluster: ", "epinio-ci"+runID)
			}
		}
	case "GKE":
		domainname := "id" + runID + "-" + gke_domain
		CleanupDNS(aws_zone_id, domainname)
		exists := ListCluster(runID, pcp)
		if exists == true {
			fmt.Println("Deleting GKE cluster ...")
			out, err := proc.RunW("gcloud", "container", "clusters", "delete", "epinioci"+runID, "--zone", gke_zone, "--quiet")
			if err != nil {
				fmt.Println("Error: ", err, out)
			} else {
				fmt.Println("Deleted GKE cluster: ", "epinioci"+runID)
			}
		}
	}
}
