package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

/*
   Example vast api payload
   {
       "cluster": "",
       "cluster_id": 1,
       "grace_period": "7 00:00:00",
       "guid": "",
       "hard_limit": 11000000000,
       "hard_limit_inodes": 10100000,
       "id": 1441,
       "internal": false,
       "name": "",
       "path": "",
       "pretty_grace_period": " 7 days",
       "pretty_grace_period_expiration": null,
       "pretty_state": "OK",
       "soft_limit": 10000000000,
       "soft_limit_inodes": 10000000,
       "state": "OK",
       "sync_state": "SYNCHRONIZED",
       "tenant_id": 1,
       "time_to_block": null,
       "title": "",
       "url": "",
       "used_capacity": 1038804300,
       "used_capacity_tb": 0.001,
       "used_effective_capacity": 1043824640,
       "used_effective_capacity_tb": 0.001,
       "used_inodes": 282
   },
*/

type vastQuota struct {
	ID                    int    `json:"id"`
	HardLimit             int    `json:"hard_limit"`
	HardLimitInodes       int    `json:"hard_limit_inodes"`
	Path                  string `json:"path"`
	GracePeriod           string `json:"pretty_grace_period"`
	SoftLimit             int    `json:"soft_limit"`
	SoftLimitInodes       int    `json:"soft_limit_inodes"`
	State                 string `json:"state"`
	SyncState             string `json:"sync_state"`
	UsedCapacity          int    `json:"used_capacity"`
	UsedEffectiveCapacity int    `json:"used_effective_capacity"`
	UsedInodes            int    `json:"used_inodes"`
}

func init() {
	viper.SetConfigName("iquota")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/iquota/")
}

func fetchQuotaReport(host, user, password string) ([]vastQuota, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/api/quotas/", host), nil)
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(user, password)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch vast quota with HTTP status code: %d", res.StatusCode)
	}

	rawJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var vq []vastQuota
	err = json.Unmarshal(rawJson, &vq)
	if err != nil {
		return nil, err
	}

	return vq, nil
}

func setDefaultUserQuota(path, host, user, password string) error {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}

	payload := map[string]interface{}{
		"name":              path,
		"path":              path,
		"grace_period":      "7 00:00:00",
		"soft_limit":        10000000000,
		"hard_limit":        11000000000,
		"soft_limit_inodes": 10000000,
		"hard_limit_inodes": 10100000,
		"create_dir":        "False",
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/api/quotas/", host), bytes.NewBuffer(b))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(user, password)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 201 {
		rawBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("Failed to create vast quota for path %s: %s", path, string(rawBody))
	}

	return nil
}

func checkAndSetUserQuotas(host, user, password string) {
	if len(viper.GetString("home_dir")) == 0 {
		log.Fatalf("You must set the home_dir config value")
	}

	log.Infof("Checking all directories under: %s", viper.GetString("home_dir"))

	quotas, err := fetchQuotaReport(host, user, password)
	if err != nil {
		log.Fatalf("Failed to fetch all quotas from vast: %s", err)
	}

	if len(quotas) <= 20 {
		log.Fatalf("Something is a bit off, we got an very small number of quotas so we're going to bail here")
	}

	qmap := make(map[string]bool)
	for _, q := range quotas {
		qmap[q.Path] = true
	}

	files, err := ioutil.ReadDir(viper.GetString("home_dir"))
	if err != nil {
		log.Fatalf("Failed to list user directories: %s", err)
	}

	count := 0
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		if file.Name() == "." || file.Name() == ".." {
			continue
		}

		abspath := path.Join(viper.GetString("home_dir"), file.Name())

		if _, ok := qmap[abspath]; ok {
			continue
		}

		log.Infof("Setting new user quota on path: %s", abspath)
		err := setDefaultUserQuota(abspath, host, user, password)
		if err != nil {
			log.Errorf("%s", err)
		} else {
			count++
		}
	}

	if count == 0 {
		log.Infof("No directories found in %s without a quota set", viper.GetString("home_dir"))
	} else {
		log.Infof("Successfully set %d quotas", count)
	}
}

func main() {
	var (
		host = kingpin.Flag(
			"host",
			"Hostname of vast server",
		).Required().Envar("VAST_HOST").String()

		vastUser = kingpin.Flag(
			"user",
			"Vast user",
		).Default("quota-reporter").Envar("VAST_USER").String()

		vastPass = kingpin.Flag(
			"password",
			"Vast password",
		).Envar("VAST_PASSWORD").String()

		expire = kingpin.Flag(
			"expire",
			"Cache expire time",
		).Default("500").Envar("VAST_EXPIRE").Int()

		debug     = kingpin.Flag("debug", "enable debug mode").Default("false").Bool()
		userCheck = kingpin.Flag("user-check", "check and set user home directories").Default("false").Bool()
	)

	viper.ReadInConfig()
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	if *userCheck {
		checkAndSetUserQuotas(*host, *vastUser, *vastPass)
		return
	}

	quotas, err := fetchQuotaReport(*host, *vastUser, *vastPass)
	if err != nil {
		log.Fatalf("Failed to fetch quota report from vast: %s", err)
	}

	cache := &iquota.Cache{Expire: *expire}

	for _, q := range quotas {
		iq := &iquota.Quota{
			Path:            q.Path,
			GracePeriod:     q.GracePeriod,
			HardLimit:       q.HardLimit,
			SoftLimit:       q.SoftLimit,
			Used:            q.UsedEffectiveCapacity,
			HardLimitInodes: q.HardLimitInodes,
			SoftLimitInodes: q.SoftLimitInodes,
			UsedInodes:      q.UsedInodes,
		}

		err := cache.SetDirectoryQuotaCache(q.Path, iq)
		if err != nil {
			log.WithFields(log.Fields{
				"path":  q.Path,
				"error": err,
			}).Error("Failed to set vast directory quota cache in redis")
		}
	}
}
