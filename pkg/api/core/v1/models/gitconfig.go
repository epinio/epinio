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

package models

// GitconfigsMatchResponse contains the list of names for matching git configurations
type GitconfigsMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// GitconfigCreateRequest contains the data for a new git configuration.
type GitconfigCreateRequest struct {
	ID           string      `json:"id,omitempty"`
	URL          string      `json:"url,omitempty"`
	Provider     GitProvider `json:"provider,omitempty"`
	UserOrg      string      `json:"userorg,omitempty"`
	Repository   string      `json:"repository,omitempty"`
	SkipSSL      bool        `json:"skipssl,omitempty"`
	Username     string      `json:"username,omitempty"`
	Password     string      `json:"password,omitempty"`
	Certificates []byte      `json:"certs,omitempty"`
}

// Gitconfig contains the public parts of git.Configuration
// Password and cert data are private and excluded.
// TODO : Track creating user of the config.
type Gitconfig struct {
	Meta       MetaLite    `json:"meta,omitempty"`
	URL        string      `json:"url,omitempty"`
	Provider   GitProvider `json:"provider,omitempty"`
	Username   string      `json:"username,omitempty"`
	UserOrg    string      `json:"userorg,omitempty"`
	Repository string      `json:"repository,omitempty"`
	SkipSSL    bool        `json:"skipssl,omitempty"`
	// Password    string - Private, excluded
	// Certificate []byte - Private, excluded
}

type GitconfigList []Gitconfig

// Implement the Sort interface for gitconfig slices
// Gitconfigs are sorted by their url, user/org, and repo, in this order.

// Len (Sort interface) returns the length of the GitconfigList
func (al GitconfigList) Len() int {
	return len(al)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the GitconfigList
func (al GitconfigList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the GitconfigList and returns true if the condition holds, and
// else false.
func (al GitconfigList) Less(i, j int) bool {
	return (al[i].URL < al[j].URL) ||
		((al[i].URL == al[j].URL) &&
			(al[i].UserOrg < al[j].UserOrg)) ||
		((al[i].URL == al[j].URL) &&
			(al[i].UserOrg == al[j].UserOrg) &&
			(al[i].Repository < al[j].Repository))
}
