// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
	"github.com/ubccr/kerby/khttp"
)

const (
	RESOURCE_USER_QUOTA  = "/quota/user"
	RESOURCE_GROUP_QUOTA = "/quota/group"
	RESOURCE_OVER_QUOTA  = "/quota/exceeded"
	LONG_FORMAT          = "%-40s%-12s%15s%10s%10s%10s%10s%12s\n"
	SHORT_FORMAT         = "%-40s%-12s%15s%10s%10s%12s\n"
)

var (
	cyan   = color.New(color.FgCyan)
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	blue   = color.New(color.FgBlue)
)

type Filesystem struct {
	Host       string
	Path       string
	MountPoint string
	UserQuota  bool
	GroupQuota bool
}

type QuotaClient struct {
	Verbose     bool
	Group       bool
	User        bool
	Default     bool
	Long        bool
	FullPath    bool
	OverQuota   bool
	UserFilter  string
	GroupFilter string
	Filesystem  string
	certPool    *x509.CertPool
}

func (f *Filesystem) String() string {
	var buf bytes.Buffer
	if len(f.Host) > 0 {
		buf.WriteString(fmt.Sprintf("%s:", f.Host))
	}
	buf.WriteString(f.Path)
	return buf.String()
}

func (f *Filesystem) ShortString() string {
	if len(f.MountPoint) > 0 {
		return f.MountPoint
	}

	return f.Path
}

func (c *QuotaClient) format() string {
	if c.Long {
		return LONG_FORMAT
	}

	return SHORT_FORMAT
}

func (c *QuotaClient) printFilesystem(fs *Filesystem) {
	if c.FullPath {
		fmt.Printf("%s\n", fs)
		return
	}

	fmt.Printf("%s\n", fs.ShortString())
}

func (c *QuotaClient) fetchQuota(url string) (*iquota.QuotaRestResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: c.certPool}}

	// XXX should we default to this? seems a bit rash? Perhaps make this a config option
	if c.certPool == nil {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	t := &khttp.Transport{Next: tr}
	client := &http.Client{Transport: t}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusInternalServerError {
		return nil, fmt.Errorf("Failed to fetch user quota with HTTP status code: %d", res.StatusCode)
	}

	rawJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusBadRequest {
		ierr := &iquota.IsiError{}
		err = json.Unmarshal(rawJson, ierr)
		if err != nil {
			return nil, err
		}
		return nil, ierr
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch user quota with HTTP status code: %d", res.StatusCode)
	}

	qr := &iquota.QuotaRestResponse{}
	err = json.Unmarshal(rawJson, qr)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

func (c *QuotaClient) parseMtab() ([]*Filesystem, error) {
	mounts := make([]*Filesystem, 0)

	defaultFs := viper.GetStringMapString("filesystems")

	mtab, err := os.Open("/etc/mtab")
	if err != nil {
		return nil, err
	}
	defer mtab.Close()

	scanner := bufio.NewScanner(mtab)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if fields[2] == "nfs" || fields[2] == "nfs4" {
			parts := strings.Split(fields[0], ":")
			fs := &Filesystem{
				Host:       parts[0],
				Path:       parts[1],
				MountPoint: fields[1],
				UserQuota:  true,
				GroupQuota: true,
			}

			defaults, ok := defaultFs[fs.Path]
			if ok {
				fs.UserQuota = strings.Contains(defaults, "user")
				fs.GroupQuota = strings.Contains(defaults, "group")
				mounts = append(mounts, fs)
			} else if len(defaultFs) == 0 && strings.HasPrefix(fs.Path, "/ifs") {
				// XXX only include isilon mounts. Will this always be /ifs?
				mounts = append(mounts, fs)
			}
		} else if fields[2] == "panfs" {
			fs := &Filesystem{
				Host:       fields[0],
				Path:       "/panasas",
				MountPoint: fields[1],
				UserQuota:  true,
				GroupQuota: true,
			}

			defaults, ok := defaultFs[fs.Path]
			if ok {
				fs.UserQuota = strings.Contains(defaults, "user")
				fs.GroupQuota = strings.Contains(defaults, "group")
				mounts = append(mounts, fs)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func (c *QuotaClient) printHeader(label string) {
	if c.Long {
		fmt.Printf(c.format(), "Filesystem ", label, "files", "logical", "physical", "soft", "hard", "grace ")
		return
	}

	fmt.Printf(c.format(), "Filesystem ", label, "files", "used", "limit", "grace ")
}

func (c *QuotaClient) printQuota(fs *Filesystem, name string, quota *iquota.Quota) {
	fsPath := ""
	if fs.Path != quota.Path {
		fsPath = quota.Path
	}

	now := time.Now()
	graceTime := now.Add(time.Duration(quota.Threshold.SoftGrace) * time.Second)
	var grace string

	printer := cyan
	if quota.Threshold.SoftExceeded {
		printer = red
		grace = humanize.RelTime(
			time.Unix(int64(quota.Threshold.SoftLastExceeded), 0).Add(time.Duration(quota.Threshold.SoftGrace)*time.Second),
			now,
			"ago",
			"")
	} else if quota.Threshold.SoftGrace == 0 {
		grace = ""
	} else {
		grace = humanize.RelTime(graceTime, now, "", "")
	}

	soft := ""
	hard := ""
	if quota.Threshold.Soft > 0 {
		soft = humanize.Bytes(uint64(quota.Threshold.Soft))
	}
	if quota.Threshold.Hard > 0 {
		hard = humanize.Bytes(uint64(quota.Threshold.Hard))
	}

	if c.Long {
		printer.Printf(c.format(),
			fsPath,
			name,
			humanize.Comma(int64(quota.Usage.Inodes)),
			humanize.Bytes(uint64(quota.Usage.Logical)),
			humanize.Bytes(uint64(quota.Usage.Physical)),
			soft,
			hard,
			grace)

		return
	}

	printer.Printf(c.format(),
		fsPath,
		name,
		humanize.Comma(int64(quota.Usage.Inodes)),
		humanize.Bytes(uint64(quota.Usage.Logical)),
		soft,
		grace)
}

func (c *QuotaClient) printDefaultQuota(quota *iquota.Quota) {
	now := time.Now()
	graceTime := now.Add(time.Duration(quota.Threshold.SoftGrace) * time.Second)
	grace := humanize.RelTime(graceTime, now, "", "")

	if c.Long {
		yellow.Printf(c.format(),
			"",
			"(default)",
			"",
			"",
			"",
			humanize.Bytes(uint64(quota.Threshold.Soft)),
			humanize.Bytes(uint64(quota.Threshold.Hard)),
			grace)

		return
	}

	yellow.Printf(c.format(),
		"",
		"(default)",
		"",
		"",
		humanize.Bytes(uint64(quota.Threshold.Soft)),
		grace)
}

func (c *QuotaClient) printUserQuota(username string, mounts []*Filesystem) {
	fmt.Printf("User quotas:\n")
	c.printHeader("user")
	for _, fs := range mounts {
		if !fs.UserQuota {
			logrus.Warn("User quota reporting disabled for filesystem: ", fs)
			continue
		}
		params := url.Values{}
		params.Add("user", username)
		params.Add("path", fs.Path)

		apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), RESOURCE_USER_QUOTA, params.Encode())

		qr, err := c.fetchQuota(apiUrl)
		if err != nil {
			if ierr, ok := err.(*iquota.IsiError); ok {
				if ierr.Code == "AEC_NOT_FOUND" {
					logrus.Warn("Invalid user: ", username)
				} else if ierr.Message == "Access denied" {
					logrus.Fatal("You must be an admin user to peform this operation.")
				}
			}

			if strings.Contains(err.Error(), "No Kerberos credentials available") {
				logrus.Fatal("No Kerberos credentials available. Please run kinit")
			}

			logrus.Warn(err)
			return
		}

		if len(qr.Quotas) == 0 && qr.Default == nil {
			if c.Verbose {
				c.printFilesystem(fs)
				fmt.Printf("   No quota defined.\n")

			} else {
				logrus.Warn("No quotas set for filesystem: ", fs)
			}

			continue
		}

		c.printFilesystem(fs)
		if qr.Default != nil && c.Default {
			c.printDefaultQuota(qr.Default)
		}
		for _, quota := range qr.Quotas {
			c.printQuota(fs, username, quota)
		}
	}
}

func (c *QuotaClient) printGroupQuota(username string, mounts []*Filesystem) {
	fmt.Printf("Group quotas:\n")
	c.printHeader("group")
	group := c.GroupFilter

	for _, fs := range mounts {
		if !fs.GroupQuota {
			logrus.Warn("Group quota reporting disabled for filesystem: ", fs)
			continue
		}
		params := url.Values{}
		params.Add("user", username)
		params.Add("path", fs.Path)
		if len(group) > 0 {
			params.Add("group", group)
		}

		apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), RESOURCE_GROUP_QUOTA, params.Encode())

		qr, err := c.fetchQuota(apiUrl)
		if err != nil {
			if ierr, ok := err.(*iquota.IsiError); ok {
				if ierr.Code == "AEC_NOT_FOUND" {
					logrus.Warn("Invalid group: ", group)
				} else if ierr.Message == "Access denied" {
					logrus.Fatal("You must be an admin user to peform this operation.")
				}
			} else if strings.Contains(err.Error(), "No Kerberos credentials available") {
				logrus.Fatal("No Kerberos credentials available. Please run kinit")
			} else {
				logrus.Fatal(err)
			}
			continue
		}

		if len(qr.Quotas) == 0 && qr.Default == nil {
			if c.Verbose {
				c.printFilesystem(fs)
				fmt.Printf("   No quota defined.\n")

			} else {
				logrus.Warn("No quotas set for filesystem: ", fs)
			}

			continue
		}

		c.printFilesystem(fs)
		if qr.Default != nil && c.Default {
			c.printDefaultQuota(qr.Default)
		}
		for _, quota := range qr.Quotas {
			gname := group
			if len(gname) == 0 && quota.Persona != nil {
				gname = quota.Persona.Name
			}
			c.printQuota(fs, gname, quota)
		}
	}
}

func (c *QuotaClient) exportOverQuota(mounts []*Filesystem) {
	params := url.Values{}
	if len(mounts) == 1 {
		params.Add("path", mounts[0].Path)
	}

	apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), RESOURCE_OVER_QUOTA, params.Encode())

	qr, err := c.fetchQuota(apiUrl)
	if err != nil {
		if ierr, ok := err.(*iquota.IsiError); ok {
			if ierr.Message == "Access denied" {
				logrus.Fatal("You must be an admin user to peform this operation.")
			}
		}

		if strings.Contains(err.Error(), "No Kerberos credentials available") {
			logrus.Fatal("No Kerberos credentials available. Please run kinit")
		}

		logrus.Fatal(err)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(qr.Quotas); err != nil {
		logrus.Fatal(err)
	}
}

func (c *QuotaClient) Run() {
	uid, err := user.Current()
	if err != nil {
		logrus.Fatal("Failed to determine user information: ", err)
	}

	username := uid.Username
	if len(c.UserFilter) != 0 {
		username = c.UserFilter
	}

	// XXX ignore mtab parsing errors for now?
	mounts, err := c.parseMtab()
	if err != nil {
		logrus.Warn("Failed to parse /etc/mtab: ", err)
	}

	path := c.Filesystem

	if len(path) == 0 && len(mounts) == 0 {
		logrus.Fatal("No path given and no nfs mounts detected. Please provide a path")
	}

	if len(path) > 0 {
		fs := &Filesystem{Path: path}
		for _, f := range mounts {
			if fs.Path == f.Path || fs.MountPoint == f.MountPoint {
				fs = f
				break
			}
		}
		mounts = []*Filesystem{fs}
	}

	if c.OverQuota {
		c.exportOverQuota(mounts)
		return
	}

	if !c.Group && len(c.GroupFilter) == 0 && (c.User || len(c.UserFilter) > 0) {
		c.printUserQuota(username, mounts)
	} else if !c.User && len(c.UserFilter) == 0 && (c.Group || len(c.GroupFilter) > 0) {
		c.printGroupQuota(username, mounts)
	} else if (c.User || len(c.UserFilter) > 0) && (c.Group || len(c.GroupFilter) > 0) {
		c.printUserQuota(username, mounts)
		fmt.Println()
		c.printGroupQuota(username, mounts)
	} else {
		c.printUserQuota(username, mounts)
	}
}
