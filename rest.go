// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package iquota

type QuotaParams struct {
	User  string `schema:"user"`
	Group string `schema:"group"`
	Type  string `schema:"type"`
	Path  string `schema:"path"`
}

type QuotaRestResponse struct {
	Default *Quota   `json:"default"`
	Quotas  []*Quota `json:"quotas"`
}

type IQuota struct {
	Path            string `json:"path"`
	GracePeriod     string `json:"pretty_grace_period"`
	HardLimit       int    `json:"hard_limit"`
	SoftLimit       int    `json:"soft_limit"`
	Used            int    `json:"used"`
	HardLimitInodes int    `json:"hard_limit_inodes"`
	SoftLimitInodes int    `json:"soft_limit_inodes"`
	UsedInodes      int    `json:"used_inodes"`
}
