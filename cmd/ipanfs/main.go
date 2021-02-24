// Copyright 2020 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// cache panfs user and group quotas
package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
	"golang.org/x/crypto/ssh"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

type PanfsRoot struct {
	Pasroot xml.Name  `xml:"pasxml"`
	Volumes []*Volume `xml:"volumes>volume"`
}

type Volume struct {
	Name string `xml:"name"`
	Used string `xml:"spaceUsedGB"`
	Hard string `xml:"hardQuotaGB"`
	Soft string `xml:"softQuotaGB"`
}

var (
	PanfsQuotaCmd        = "userquota usage -output tab"
	PanfsLoginEndpoint   = "/pasxml/login"
	PanfsVolumesEndpoint = "/pasxml/volumes"
	client               *http.Client

	prefix = kingpin.Flag(
		"prefix",
		"Path prefix for mount point of panfs",
	).Default("/panasas").Envar("IPANFS_PREFIX").String()

	address = kingpin.Flag(
		"address",
		"Address of panfs server",
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

func init() {
	viper.SetConfigName("iquota")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/iquota/")
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Failed to create cookie jar: %s", err)
	}
	client = &http.Client{Jar: jar, Transport: tr}
}

func login() error {
	req, err := http.NewRequest("GET", "https://"+*address+":10635/"+PanfsLoginEndpoint, nil)

	params := req.URL.Query()
	params.Add("name", *panUser)
	params.Add("pass", *panPass)
	req.URL.RawQuery = params.Encode()

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("Failed to login to panfs with HTTP status code: %d", res.StatusCode)
	}

	return nil
}

func fetchVolumes() ([]*Volume, error) {
	err := login()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", "https://"+*address+":10635/"+PanfsVolumesEndpoint, nil)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to login to panfs with HTTP status code: %d", res.StatusCode)
	}

	rawXml, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var result PanfsRoot
	err = xml.Unmarshal(rawXml, &result)
	if err != nil {
		return nil, err
	}

	return result.Volumes, nil
}

func parseGroupQuotas(report io.Reader) (map[string]*iquota.Quota, error) {
	scanner := bufio.NewScanner(report)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "# Group Quota Usage" {
			break
		}
	}

	quotas := make(map[string]*iquota.Quota, 0)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cols := strings.Split(scanner.Text(), "\t")
		path := cols[0]
		filesUsed, _ := strconv.Atoi(cols[10])

		q, ok := quotas[path]
		if ok {
			if filesUsed > q.UsedInodes {
				// If multiple groups just take the largest one
				quotas[path].UsedInodes = filesUsed
			}
		} else {
			quotas[path] = &iquota.Quota{
				Path:       path,
				UsedInodes: filesUsed,
			}
		}
	}

	return quotas, nil
}

func runPanfsCmd(command string) (*bytes.Buffer, error) {
	authMethods := make([]ssh.AuthMethod, 0)

	if *key != "" {
		sshKey, err := ioutil.ReadFile(*key)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(sshKey)
		if err != nil {
			return nil, err
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if *panPass != "" {
		authMethods = append(authMethods, ssh.Password(*panPass))
	}

	config := &ssh.ClientConfig{
		User:            *panUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", *address+":22", config)
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

	err = sess.Run(command)
	if err != nil {
		log.Debugf("Failed to run userquota: %s", stderr.String())
		return nil, err
	}

	return &stdout, nil

}

func fetchQuotaReport() (*bytes.Buffer, error) {
	return runPanfsCmd(PanfsQuotaCmd)
}

func main() {
	viper.ReadInConfig()
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	report, err := fetchQuotaReport()
	if err != nil {
		log.Fatalf("Failed to fetch quota report from panfs: %s", err)
	}

	volumes, err := fetchVolumes()
	if err != nil {
		log.Fatalf("Failed to fetch volumes: %s", err)
	}
	if *noop {
		for _, v := range volumes {
			fmt.Printf("%#v\n", v)
		}
		fmt.Println(report.String())
		os.Exit(0)
	}

	reader := bytes.NewReader(report.Bytes())
	gquotas, err := parseGroupQuotas(reader)
	if err != nil {
		log.Fatalf("Failed to parse group quota report from panfs: %s", err)
	}

	cache := &iquota.Cache{Expire: *expire}
	for _, v := range volumes {
		path := fmt.Sprintf("%s%s", *prefix, v.Name)

		hard, err := humanize.ParseBytes(v.Hard + " GB")
		if err != nil {
			log.Errorf("Failed to parse hard quota to bytes for path %s: %s", v.Name, err)
		}
		soft, err := humanize.ParseBytes(v.Soft + " GB")
		if err != nil {
			log.Errorf("Failed to parse soft quota to bytes for path %s: %s", v.Name, err)
		}
		used, err := humanize.ParseBytes(v.Used + " GB")
		if err != nil {
			log.Errorf("Failed to parse used bytes for path %s: %s", v.Name, err)
		}
		iq := &iquota.Quota{
			Path:        path,
			GracePeriod: "7 days",
			HardLimit:   int(hard),
			SoftLimit:   int(soft),
			Used:        int(used),
		}

		q, ok := gquotas[v.Name]
		if ok {
			// XXX hack to set the number of files used from the group quota
			iq.UsedInodes = q.UsedInodes
		}

		err = cache.SetDirectoryQuotaCache(path, iq)
		if err != nil {
			log.WithFields(log.Fields{
				"path":  path,
				"error": err,
			}).Error("Failed to set panfs directory quota cache in redis")
		}
	}
}
