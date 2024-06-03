// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	munge "github.com/ubccr/gomunge"
	"github.com/ubccr/iquota"
)

const (
	QuotaEndpoint = "/quota"
	LongFormat    = "%-30s%15s%15s%15s%10s%10s%12s\n"
	ShortFormat   = "%-30s%15s%15s%15s%12s\n"
)

var (
	cyan   = color.New(color.FgCyan)
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	blue   = color.New(color.FgBlue)
)

type QuotaClient struct {
	Group       bool
	User        bool
	Long        bool
	UserFilter  string
	GroupFilter string
	Path        string
	certPool    *x509.CertPool
}

func (c *QuotaClient) format() string {
	if c.Long {
		return LongFormat
	}

	return ShortFormat
}

func (c *QuotaClient) fetchQuota(url string) ([]*iquota.Quota, error) {
	req, err := http.NewRequest("GET", url, nil)
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: c.certPool}}

	// XXX should we default to this? seems a bit rash? Perhaps make this a config option
	if c.certPool == nil {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	cred, err := munge.Encode()
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", cred)

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusInternalServerError {
		return nil, fmt.Errorf("Failed to fetch quota with HTTP status code: %d", res.StatusCode)
	} else if res.StatusCode == 404 {
		return nil, iquota.ErrNotFound
	} else if res.StatusCode == 401 {
		return nil, fmt.Errorf("You are not authorized to fetch this quota")
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch quota with HTTP status code: %d", res.StatusCode)
	}

	rawJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var quotas []*iquota.Quota
	err = json.Unmarshal(rawJson, &quotas)
	if err != nil {
		return nil, err
	}

	return quotas, nil
}

func (c *QuotaClient) printHeader() {
	if c.Long {
		fmt.Printf(c.format(), "Path ", "files", "limit", "used", "soft", "hard", "grace ")
		return
	}

	fmt.Printf(c.format(), "Path ", "files", "used", "limit", "grace ")
}

func (c *QuotaClient) printQuota(quota *iquota.Quota) {
	printer := cyan
	soft := ""
	hard := ""
	if quota.SoftLimit > 0 {
		soft = humanize.Bytes(uint64(quota.SoftLimit))
	}
	if quota.HardLimit > 0 {
		hard = humanize.Bytes(uint64(quota.HardLimit))
	}

	if c.Long {
		printer.Printf(c.format(),
			quota.Path,
			humanize.Comma(int64(quota.UsedInodes)),
			humanize.Comma(int64(quota.HardLimitInodes)),
			humanize.Bytes(uint64(quota.Used)),
			soft,
			hard,
			quota.GracePeriod)

		return
	}

	printer.Printf(c.format(),
		quota.Path,
		humanize.Comma(int64(quota.UsedInodes)),
		humanize.Bytes(uint64(quota.Used)),
		soft,
		quota.GracePeriod)
}

func (c *QuotaClient) printDirectoryQuota() {
	c.printHeader()
	params := url.Values{}
	if len(c.Path) > 0 {
		params.Add("path", c.Path)
	} else if len(c.UserFilter) > 0 {
		params.Add("user", c.UserFilter)
	} else if len(c.GroupFilter) > 0 {
		params.Add("group", c.GroupFilter)
	}

	apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), QuotaEndpoint, params.Encode())

	quotas, err := c.fetchQuota(apiUrl)
	if err != nil {
		if errors.Is(err, iquota.ErrNotFound) {
			logrus.Warn("No quotas found")
			return
		}

		logrus.Fatal(err)
		return
	}

	if len(quotas) == 0 {
		logrus.Warn("No quotas found")
		return
	}

	for _, quota := range quotas {
		c.printQuota(quota)
	}
}

func (c *QuotaClient) Run() {
	c.printDirectoryQuota()
}
