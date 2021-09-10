package route53

import (
	"encoding/json"
	"io/ioutil"
	"path"

	"github.com/epinio/epinio/acceptance/helpers/proc"
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

func CNAME(record string, value string) ChangeResourceRecordSet {
	return ChangeResourceRecordSet{
		Changes: []Change{
			{
				Action: "UPSERT",
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

func A(record string, value string) ChangeResourceRecordSet {
	return ChangeResourceRecordSet{
		Changes: []Change{
			{
				Action: "UPSERT",
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

func Upsert(zoneID string, change ChangeResourceRecordSet, dir string) (string, error) {
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
