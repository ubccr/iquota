// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
	"github.com/ubccr/kerby/khttp"
)

const (
	RESOURCE_USER_QUOTA  = "/quota/user"
	RESOURCE_GROUP_QUOTA = "/quota/group"
)

var (
	green = color.New(color.FgCyan)
	red   = color.New(color.FgRed)
)

type Filesystem struct {
	Host       string
	Path       string
	MountPoint string
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

func FetchQuota(url string) (*iquota.QuotaRestResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	//XXX fix me to use cacert
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
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

func parseMtab() ([]*Filesystem, error) {
	mounts := make([]*Filesystem, 0)

	mtab, err := os.Open("/etc/mtab")
	if err != nil {
		return nil, err
	}
	defer mtab.Close()

	scanner := bufio.NewScanner(mtab)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if fields[2] == "nfs" {
			parts := strings.Split(fields[0], ":")
			fs := &Filesystem{Host: parts[0], Path: parts[1], MountPoint: fields[1]}
			mounts = append(mounts, fs)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func printUserQuota(c *cli.Context, username string, mounts []*Filesystem) {
	fmt.Printf("Disk quotas for user %s:\n", username)
	fmt.Printf("%-12s%12s%12s%12s%12s\n", "Filesystem ", "files", "used", "limit", "grace")
	for _, fs := range mounts {
		params := url.Values{}
		params.Add("user", username)
		params.Add("path", fs.Path)

		apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), RESOURCE_USER_QUOTA, params.Encode())

		qr, err := FetchQuota(apiUrl)
		if err != nil {
			if ierr, ok := err.(*iquota.IsiError); ok {
				if ierr.Code == "AEC_NOT_FOUND" {
					logrus.Fatal("Invalid user: ", username)
				} else if ierr.Message == "Access denied" {
					logrus.Fatal("You must be an admin user to peform this operation.")
				}
			}

			if strings.Contains(err.Error(), "No Kerberos credentials available") {
				logrus.Fatal("No Kerberos credentials available. Please run kinit")
			}

			logrus.Fatal(err)
		}

		if len(qr.Quotas) == 0 && qr.Default == nil {
			if c.Bool("verbose") {
				fmt.Printf("%s\n", fs)
				fmt.Printf("   No quota defined.\n")

			} else {
				logrus.Warn("No quotas set for filesystem: ", fs)
			}

			continue
		}

		if len(qr.Quotas) == 0 {
			qr.Quotas = append(qr.Quotas, qr.Default)
		}

		fmt.Printf("%s\n", fs)
		for _, quota := range qr.Quotas {
            now := time.Now()
            grace := now.Add(time.Duration(quota.Threshold.SoftGrace)*time.Second)

			printer := green
			if quota.Threshold.SoftExceeded {
				printer = red
			}
			printer.Printf("%-12s%12d%12s%12s%12s\n",
				"",
				quota.Usage.Inodes,
				humanize.Bytes(uint64(quota.Usage.Logical)),
				humanize.Bytes(uint64(quota.Threshold.Soft)),
				humanize.RelTime(grace, now, "", ""))
		}
	}
}

func printGroupQuota(c *cli.Context, username string, mounts []*Filesystem) {
	fmt.Printf("Disk quotas for group\n")
	fmt.Printf("%-12s%-12s%12s%12s%12s%12s\n", "Filesystem ", "group", "files", "used", "limit", "grace")
	group := c.String("group")

	for _, fs := range mounts {
		params := url.Values{}
		params.Add("user", username)
		params.Add("path", fs.Path)
		if len(group) > 0 {
			params.Add("group", group)
		}

		apiUrl := fmt.Sprintf("%s%s?%s", viper.GetString("iquota_url"), RESOURCE_GROUP_QUOTA, params.Encode())

		qr, err := FetchQuota(apiUrl)
		if err != nil {
			if ierr, ok := err.(*iquota.IsiError); ok {
				if ierr.Code == "AEC_NOT_FOUND" {
					logrus.Fatal("Invalid group: ", group)
				} else if ierr.Message == "Access denied" {
					logrus.Fatal("You must be an admin user to peform this operation.")
				}
			}

			if strings.Contains(err.Error(), "No Kerberos credentials available") {
				logrus.Fatal("No Kerberos credentials available. Please run kinit")
			}

			logrus.Fatal(err)
		}

		if len(qr.Quotas) == 0 && qr.Default == nil {
			if c.Bool("verbose") {
				fmt.Printf("%s\n", fs)
				fmt.Printf("   No quota defined.\n")

			} else {
				logrus.Warn("No quotas set for filesystem: ", fs)
			}

			continue
		}

		if len(group) > 0 && len(qr.Quotas) == 0 {
			qr.Quotas = append(qr.Quotas, qr.Default)
		}

		fmt.Printf("%s\n", fs)
		for _, quota := range qr.Quotas {
            now := time.Now()
            grace := now.Add(time.Duration(quota.Threshold.SoftGrace)*time.Second)

			printer := green
			if quota.Threshold.SoftExceeded {
				printer = red
			}
			gname := group
			if len(gname) == 0 && quota.Persona != nil {
				gname = quota.Persona.Name
			}
			printer.Printf("%-12s%-12s%12d%12s%12s%12s\n",
				"",
				gname,
				quota.Usage.Inodes,
				humanize.Bytes(uint64(quota.Usage.Logical)),
				humanize.Bytes(uint64(quota.Threshold.Soft)),
				humanize.RelTime(grace, now, "", ""))
		}
	}
}

func QuotaClient(c *cli.Context) {
	uid, err := user.Current()
	if err != nil {
		logrus.Fatal("Failed to determine user information: ", err)
	}

	username := uid.Username
	if len(c.String("user")) != 0 {
		username = c.String("user")
	}

	// XXX ignore mtab parsing errors for now?
	mounts, err := parseMtab()
	if err != nil {
		logrus.Warn("Failed to parse /etc/mtab: ", err)
	}

	path := c.String("filesystem")

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

	if !c.Bool("g") && len(c.String("group")) == 0 && (c.Bool("u") || len(c.String("user")) > 0) {
		printUserQuota(c, username, mounts)
	} else if !c.Bool("u") && len(c.String("user")) == 0 && (c.Bool("g") || len(c.String("group")) > 0) {
		printGroupQuota(c, username, mounts)
	} else if (c.Bool("u") || len(c.String("user")) > 0) && (c.Bool("g") || len(c.String("group")) > 0) {
		printUserQuota(c, username, mounts)
		fmt.Println()
		printGroupQuota(c, username, mounts)
	} else {
		printUserQuota(c, username, mounts)
	}
}
