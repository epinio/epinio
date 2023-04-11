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

package route53

import (
	"encoding/json"
	"fmt"
	"os"
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

type RecordValues struct {
	Name   string
	Type   string
	Record string
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
	err = os.WriteFile(f, b, 0600)
	if err != nil {
		return "", err
	}
	return proc.RunW("aws", "route53", "change-resource-record-sets", "--hosted-zone-id", zoneID, "--change-batch", "file://"+f)
}

func TestDnsAnswer(zoneID string, recordName string, recordType string) (string, error) {
	return proc.RunW("aws", "route53", "test-dns-answer", "--hosted-zone-id", zoneID, "--record-name", recordName, "--record-type", recordType, "--resolver-ip", resolverIP)
}

// External delete_cluster.go needs to query an existing record, to get the actual record type for deletion
func GetRecord(zoneID string, domainname string) (RecordValues, error) {
	var r RecordValues
	b, err := proc.RunW("aws", "route53", "list-resource-record-sets", "--hosted-zone-id", zoneID, "--query", fmt.Sprintf("ResourceRecordSets[?Name == '%s']", domainname))
	if err != nil {
		return r, err
	}

	v := []ResourceRecordSet{}
	err = json.Unmarshal([]byte(b), &v)
	if err != nil {
		return r, err
	}
	if len(v) == 0 {
		return r, nil
	}

	r = RecordValues{v[0].Name, v[0].Type, v[0].ResourceRecords[0].Value}
	return r, nil
}
