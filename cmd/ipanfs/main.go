// Copyright 2020 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// cache panfs user and group quotas
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/ubccr/iquota"
	"golang.org/x/crypto/ssh"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	PanfsQuotaCmd = "userquota usage -output tab"
	SkipPaths     = []string{"/home"}
	SkipUsers     = []string{"root"}
	SkipGroups    = []string{"wheel"}
)

func skip(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}

	return false
}

func cacheGroupQuota(cache *iquota.Cache, prefix string, report io.Reader) error {
	scanner := bufio.NewScanner(report)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "# Group Quota Usage" {
			break
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cols := strings.Split(scanner.Text(), "\t")
		path := fmt.Sprintf("%s%s", prefix, cols[0])
		if skip(SkipPaths, path) {
			continue
		}

		groupname := cols[3]
		log.Debugf("Found groupname: %s", groupname)
		if strings.HasPrefix(groupname, "gid:") {
			gid := strings.ReplaceAll(groupname, "gid:", "")
			g, err := user.LookupGroupId(gid)
			if err != nil {
				groupname = gid
				log.WithFields(log.Fields{
					"path":  path,
					"group": groupname,
					"error": err,
				}).Error("Failed to lookup gidnumber")
			} else {
				groupname = g.Name
			}
		}
		if skip(SkipGroups, groupname) {
			continue
		}

		bytesUsed, _ := strconv.Atoi(cols[5])
		soft, _ := strconv.Atoi(cols[6])
		//softPct, _ := strconv.Atoi(cols[7])
		hard, _ := strconv.Atoi(cols[8])
		//hardPct, _ := strconv.Atoi(cols[9])
		filesUsed, _ := strconv.Atoi(cols[10])

		quota := &iquota.IQuota{
			Path:       path,
			HardLimit:  hard,
			SoftLimit:  soft,
			Used:       bytesUsed,
			UsedInodes: filesUsed,
		}

		err := cache.SetDirectoryQuotaCache(path, quota)
		if err != nil {
			log.WithFields(log.Fields{
				"path":  path,
				"error": err,
			}).Error("Failed to set panfs directory quota cache in redis")
		}
	}

	return nil
}

func cacheUserQuota(cache *iquota.Cache, prefix string, report io.Reader) error {
	scanner := bufio.NewScanner(report)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "# User Quota Usage" {
			break
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			if line == "# Group Quota Usage" {
				break
			}
			continue
		}

		cols := strings.Split(scanner.Text(), "\t")
		path := fmt.Sprintf("%s%s", prefix, cols[0])
		if skip(SkipPaths, path) {
			continue
		}

		username := cols[3]
		log.Debugf("Found username: %s", username)

		if strings.HasPrefix(username, "uid:") {
			uid := strings.ReplaceAll(username, "uid:", "")
			u, err := user.LookupId(uid)
			if err != nil {
				username = uid
				log.WithFields(log.Fields{
					"path":  path,
					"user":  username,
					"error": err,
				}).Error("Failed to lookup uidnumber")
			} else {
				username = u.Username
			}
		}
		if skip(SkipUsers, username) {
			continue
		}

		bytesUsed, _ := strconv.Atoi(cols[5])
		soft, _ := strconv.Atoi(cols[6])
		//softPct, _ := strconv.Atoi(cols[7])
		hard, _ := strconv.Atoi(cols[8])
		//hardPct, _ := strconv.Atoi(cols[9])
		filesUsed, _ := strconv.Atoi(cols[10])

		quota := &iquota.IQuota{
			Path:       path,
			HardLimit:  hard,
			SoftLimit:  soft,
			Used:       bytesUsed,
			UsedInodes: filesUsed,
		}

		err := cache.SetDirectoryQuotaCache(path, quota)
		if err != nil {
			log.WithFields(log.Fields{
				"path":  path,
				"error": err,
			}).Error("Failed to set panfs directory quota cache in redis")
		}
	}

	return nil
}

func fetchQuotaReport(address, username, pass, key string) (*bytes.Buffer, error) {
	authMethods := make([]ssh.AuthMethod, 0)

	if key != "" {
		key, err := ioutil.ReadFile(key)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if pass != "" {
		authMethods = append(authMethods, ssh.Password(pass))
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	err = sess.Run(PanfsQuotaCmd)
	if err != nil {
		log.Debugf("Failed to run userquota: %s", stderr.String())
		return nil, err
	}

	return &stdout, nil
}

func main() {
	var (
		prefix = kingpin.Flag(
			"prefix",
			"Path prefix for mount point of panfs",
		).Default("/panasas").Envar("IPANFS_PREFIX").String()

		address = kingpin.Flag(
			"address",
			"Address of panfs server [host:port]",
		).Required().Envar("IPANFS_ADDRESS").String()

		panUser = kingpin.Flag(
			"user",
			"SSH user",
		).Default("guest").Envar("IPANFS_USER").String()

		panPass = kingpin.Flag(
			"password",
			"SSH password",
		).Envar("IPANFS_PASSWORD").String()

		key = kingpin.Flag(
			"key",
			"SSH key",
		).Envar("IPANFS_KEY").String()

		expire = kingpin.Flag(
			"expire",
			"Cache expire time",
		).Default("500").Envar("IPANFS_EXPIRE").Int()

		debug = kingpin.Flag("debug", "enable debug mode").Default("false").Bool()
		noop  = kingpin.Flag("noop", "Dump quota report from panfs and exit").Default("false").Bool()
	)

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	out, err := fetchQuotaReport(*address, *panUser, *panPass, *key)
	if err != nil {
		log.Fatalf("Failed to fetch quota report from panfs: %s", err)
	}

	if *noop {
		fmt.Println(out.String())
		os.Exit(0)
	}

	cache := &iquota.Cache{Expire: *expire}

	reader := bytes.NewReader(out.Bytes())

	err = cacheUserQuota(cache, *prefix, reader)
	if err != nil {
		log.Fatalf("Failed to parse user quota report from panfs: %s", err)
	}

	reader.Seek(0, 0)

	err = cacheGroupQuota(cache, *prefix, reader)
	if err != nil {
		log.Fatalf("Failed to parse group quota report from panfs: %s", err)
	}
}
