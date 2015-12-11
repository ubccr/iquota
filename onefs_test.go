// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package iquota

import (
	"os"
	"testing"
)

func newClient() *Client {
	host := os.Getenv("ONEFS_TEST_HOST")
	user := os.Getenv("ONEFS_TEST_USER")
	pass := os.Getenv("ONEFS_TEST_PASSWD")

	return NewClient(host, 8080, user, pass, "")
}

func TestLogin(t *testing.T) {
	c := newClient()
	sess, err := c.NewSession()
	if err != nil {
		t.Error(err)
	}

	if len(sess) == 0 {
		t.Error(err)
	}
}
