package route53

import (
	"encoding/json"
	"io/ioutil"
	"path"

	"github.com/epinio/epinio/acceptance/helpers/proc"
)

const (
	// Uses Google's DNS because it's the most widely deployed and used one
	resolverIP = "8.8.8.8"
)

type ResourceRecord struct {
	Value string `json:"Value"`
}

type ResourceRecordSet struct {
	Name            string           `json:"Name"`
	Type            string           `json:"Type"`
	TTL             int              `json:"TTL"`
	ResourceRecords []ResourceRecord `json:"ResourceRecords"`
}

type Change struct {
	Action            string            `json:"Action"`
	ResourceRecordSet ResourceRecordSet `json:"ResourceRecordSet"`
}

type ChangeResourceRecordSet struct {
	Changes []Change `json:"Changes"`
}

type DNSAnswer struct {
	Nameserver   string   `json:"Nameserver"`
	RecordName   string   `json:"RecordName"`
	RecordType   string   `json:"RecordType"`
	RecordData   []string `json:"RecordData"`
	ResponseCode string   `json:"ResponseCode"`
	Protocol     string   `json:"Protocol"`
}

func CNAME(record string, value string, action string) ChangeResourceRecordSet {
	return ChangeResourceRecordSet{
		Changes: []Change{
			{
				Action: action,
				ResourceRecordSet: ResourceRecordSet{
					Name: record,
					Type: "CNAME",
					TTL:  120,
					ResourceRecords: []ResourceRecord{
						{Value: value},
					},
				},
			},
		},
	}
}

func A(record string, value string, action string) ChangeResourceRecordSet {
	return ChangeResourceRecordSet{
		Changes: []Change{
			{
				Action: action,
				ResourceRecordSet: ResourceRecordSet{
					Name: record,
					Type: "A",
					TTL:  60,
					ResourceRecords: []ResourceRecord{
						{Value: value},
					},
				},
			},
		},
	}
}

func Update(zoneID string, change ChangeResourceRecordSet, dir string) (string, error) {
	b, err := json.MarshalIndent(change, "", " ")
	if err != nil {
		return "", err
	}

	f := path.Join(dir, "zone.json")
	err = ioutil.WriteFile(f, b, 0600)
	if err != nil {
		return "", err
	}
	return proc.RunW("aws", "route53", "change-resource-record-sets", "--hosted-zone-id", zoneID, "--change-batch", "file://"+f)
}

func TestDnsAnswer(zoneID string, recordName string, recordType string) (string, error) {
	return proc.RunW("aws", "route53", "test-dns-answer", "--hosted-zone-id", zoneID, "--record-name", recordName, "--record-type", recordType, "--resolver-ip", resolverIP)
}
