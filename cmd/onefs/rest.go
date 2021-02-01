// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package onefs

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
