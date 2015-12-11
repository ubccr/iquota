// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package iquota

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
)

const (
	RESOURCE_QUOTAS = "/platform/1/quota/quotas"

	QUOTA_TYPE_DIRECTORY     = "directory"
	QUOTA_TYPE_USER          = "user"
	QUOTA_TYPE_GROUP         = "group"
	QUOTA_TYPE_DEFAULT_USER  = "default-user"
	QUOTA_TYPE_DEFAULT_GROUP = "default-group"
)

// A persona consists of either a 'type' and 'name' or a 'ID'.
type Persona struct {
	// "Serialized form (e.g. 'UID:0', 'USER:name', 'GID:0', 'GROUP:wheel', 'SID:S-1-1').
	Id string `json:"id"`

	// Persona name, must be combined with type
	Name string `json:"name"`

	// Type of persona when using name "user", "group", "wellknown"
	Type string `json:"type"`
}

// Quota thresholds
type Threshold struct {
	// Usage bytes at which notifications will be sent but writes will not be denied
	Advisory int `json:"advisory"`

	// True if the advisory threshold has been hit
	AdvisoryExceeded bool `json:"advisory_exceeded"`

	// Time at which advisory threshold was hit
	AdvisoryLast_exceeded int `json:"advisory_last_exceeded"`

	// Usage bytes at which further writes will be denied
	Hard int `json:"hard"`

	// True if the hard threshold has been hit
	HardExceeded bool `json:"hard_exceeded"`

	// Time at which hard threshold was hit
	HardLastExceeded int `json:"hard_last_exceeded"`

	// Usage bytes at which notifications will be sent and soft grace time will be started
	Soft int `json:"soft"`

	// True if the soft threshold has been hit
	SoftExceeded bool `json:"soft_exceeded"`

	// Time in seconds after which the soft threshold has been hit before writes will be denied
	SoftGrace int `json:"soft_grace"`

	// Time at which soft threshold was hit
	SoftLastExceeded int `json:"soft_last_exceeded"`
}

// Quota usage
type Usage struct {
	// Number of inodes (filesystem entities) used by governed data
	Inodes int `json:"inodes"`

	// Apparent bytes used by governed data
	Logical int `json:"logical"`

	// Bytes used for governed data and filesystem overhead
	Physical int `json:"physical"`
}

// Isilon SmartQuota
type Quota struct {
	// If true, SMB shares using the quota directory see the quota thresholds
	// as share size
	Container bool `json:"container"`

	// True if the quota provides enforcement, otherwise a accounting quota
	Enforced bool `json:"enforced"`

	// The system ID given to the quota
	Id string `json:"id"`

	// If true, quota governs snapshot data as well as head data
	IncludeSnapshots bool `json:"include_snapshots"`

	// Summary of notifications: 'custom' indicates one or more notification
	// rules available from the notifications sub-resource; 'default' indicates
	// system default rules are used; 'disabled' indicates that no
	// notifications will be used for this quota.
	Notifications string `json:"notifications"`

	// For user and group quotas, true if the quota is linked and controlled by
	// a parent default-* quota. Linked quotas cannot be modified until they
	// are unlinked
	Linked bool `json:"linked"`

	// The /ifs path governed
	Path string `json:"path"`

	// True if the accounting is accurate on the quota.  If false, this quota
	// is waiting on completion of a QuotaScan job
	Ready bool `json:"ready"`

	// If true, thresholds apply to data plus filesystem overhead required to
	// store the data (i.e. 'physical' usage)
	ThresholdsIncludeOverhead bool `json:"thresholds_include_overhead"`

	// The type of quota "directory", "user", "group", "default-user",
	// "default-group"
	Type string `json:"type"`

	// Nested types
	Persona   *Persona   `json:"persona"`
	Threshold *Threshold `json:"thresholds"`
	Usage     *Usage     `json:"usage"`
}

// Response returned from Quota endpoint
type QuotaResponse struct {
	// List of errors
	Errors []*IsiError `json:"errors"`

	// List of Quotas
	Quotas []*Quota `json:"quotas"`

	// Continue returning results from previous call using this token (token
	// should come from the previous call, resume cannot be used with other
	// options).
	Resume string `json:"resume"`
}

// Fetch Quota
func (c *Client) FetchQuota(path, qtype, persona string, resolveNames bool) (*QuotaResponse, error) {
	params := url.Values{}
	params.Add("path", path)
	params.Add("type", qtype)
	params.Add("persona", persona)
	if resolveNames {
		params.Add("resolve_names", "true")
	}

	apiUrl := fmt.Sprintf("%s?%s", c.Url(RESOURCE_QUOTAS), params.Encode())

	req, err := c.getRequest(apiUrl)
	if err != nil {
		return nil, err
	}

	client := c.httpClient()

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 500 {
		return nil, fmt.Errorf("Failed to fetch user quota with HTTP status code: %d", res.StatusCode)
	}

	rawJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var qres QuotaResponse
	err = json.Unmarshal(rawJson, &qres)
	if err != nil {
		return nil, err
	}

	if qres.Errors != nil {
		// Only return first error for now
		return nil, qres.Errors[0]
	}

	return &qres, nil
}

// Fetch User Quota
func (c *Client) FetchUserQuota(path, user string) (*QuotaResponse, error) {
	persona := fmt.Sprintf("USER:%s", user)
	return c.FetchQuota(path, "user", persona, true)
}

// Fetch Group Quota
func (c *Client) FetchGroupQuota(path, group string) (*QuotaResponse, error) {
	persona := fmt.Sprintf("GROUP:%s", group)
	return c.FetchQuota(path, "group", persona, true)
}
