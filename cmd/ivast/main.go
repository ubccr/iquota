package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/ubccr/iquota"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

/*
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

		debug = kingpin.Flag("debug", "enable debug mode").Default("false").Bool()
	)

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
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
