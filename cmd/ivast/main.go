package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/dustin/go-humanize"
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

const (
	LongFormat = "%-30s%15s%15s%10s%10s%12s\n"
)

var (
	DefaultUserQuotaLimit = uint64(25000000000)   //  25G
	DefaultUserFilesLimit = uint64(10000000)      //  10M
	DefaultQuotaLimit     = uint64(1000000000000) //   1T
	DefaultFilesLimit     = uint64(200000000)     // 200M

	debug = kingpin.Flag("debug", "enable debug mode").Default("false").Bool()

	vastHost = kingpin.Flag(
		"host",
		"Hostname of vast server",
	).Default("vast-mgt.cbls.ccr.buffalo.edu").Envar("VAST_HOST").String()

	vastUser = kingpin.Flag(
		"user",
		"Vast user",
	).Default("quota-reporter").Envar("VAST_USER").String()

	vastPass = kingpin.Flag(
		"password",
		"Vast password",
	).Envar("VAST_PASSWORD").String()

	cmdGetQuota  = kingpin.Command("get-quota", "Get VAST quota")
	getQuotaPath = cmdGetQuota.Flag(
		"path",
		"Path to directory",
	).Required().String()

	cmdSetQuota = kingpin.Command("set-quota", "Set VAST quota")
	quotaPath   = cmdSetQuota.Flag(
		"path",
		"Path to directory for setting quota",
	).Required().String()

	quotaLimit = cmdSetQuota.Flag(
		"limit",
		"The quota size limit",
	).String()

	cmdCache = kingpin.Command("cache", "Cache VAST quotas")
	expire   = cmdCache.Flag(
		"expire",
		"Cache expire time",
	).Default("500").Envar("VAST_EXPIRE").Int()

	cmdUserCheck = kingpin.Command("user-check", "Check and set user home directories")
)

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

func fetchQuotaReport(dirPath string) ([]vastQuota, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/api/quotas/", *vastHost), nil)
	if len(dirPath) > 0 {
		params := req.URL.Query()
		params.Add("path", dirPath)
		req.URL.RawQuery = params.Encode()
	}

	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(*vastUser, *vastPass)
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

func createVastDirectoryQuota(path string, softLimit, filesLimit uint64) error {
	apiUrl := fmt.Sprintf("https://%s/api/quotas/", *vastHost)
	return setVastDirectoryQuota("POST", apiUrl, path, softLimit, filesLimit)
}

func updateVastDirectoryQuota(quota vastQuota, path string, softLimit, filesLimit uint64) error {
	apiUrl := fmt.Sprintf("https://%s/api/quotas/%d/", *vastHost, quota.ID)
	return setVastDirectoryQuota("PATCH", apiUrl, path, softLimit, filesLimit)
}

func setVastDirectoryQuota(verb, apiUrl, path string, softLimit, filesLimit uint64) error {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}

	payload := map[string]interface{}{
		"name":              path,
		"path":              path,
		"grace_period":      "7 00:00:00",
		"soft_limit":        softLimit,
		"hard_limit":        softLimit + 1000000000, // add 1G
		"soft_limit_inodes": filesLimit,
		"hard_limit_inodes": filesLimit + 100000, // add 100K
		"create_dir":        "False",
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(verb, apiUrl, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(*vastUser, *vastPass)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 201 || res.StatusCode == 200 {
		return nil
	}

	rawBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	return fmt.Errorf("Failed with code %d createing vast quota for path %s: %s", res.StatusCode, path, string(rawBody))
}

func setDirectoryQuota() {
	dirPath := strings.TrimSuffix(*quotaPath, "/")
	if len(dirPath) == 0 {
		log.Fatalf("Please provide a directory path (--path)")
	}

	if !strings.HasPrefix(dirPath, "/") {
		log.Fatalf("Paths must be absolute: %s", *quotaPath)
	}

	finfo, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Directory does not exist: %s", *quotaPath)
		}

		log.Fatalf("Failed to check directory path %s: %s", *quotaPath, err)
	}

	if !finfo.IsDir() {
		log.Fatalf("Path exist but is not a directory: %s", *quotaPath)
	}

	bytes := DefaultQuotaLimit
	if len(*quotaLimit) > 0 {
		bytes, err = humanize.ParseBytes(*quotaLimit)
		if err != nil {
			log.Fatalf("Invalid quota limit value: %s", err)
		}
	}

	quotas, err := fetchQuotaReport(dirPath)
	if err != nil {
		log.Fatalf("Failed to check quota report for %s: %s", dirPath, err)
	}

	if len(quotas) == 0 {
		err = createVastDirectoryQuota(dirPath, bytes, DefaultFilesLimit)
	} else if len(quotas) == 1 {
		err = updateVastDirectoryQuota(quotas[0], dirPath, bytes, DefaultFilesLimit)
	} else {
		log.Fatalf("Failed to set quota. More than one already exists: ", dirPath)
	}

	if err != nil {
		log.Fatalf("Failed to set quota on %s: %s", dirPath, err)
	}

	fmt.Printf("Successfully set %s quota on %s\n", humanize.Bytes(bytes), dirPath)
}

func checkAndSetUserQuotas() {
	if len(viper.GetString("home_dir")) == 0 {
		log.Fatalf("You must set the home_dir config value")
	}

	log.Infof("Checking all directories under: %s", viper.GetString("home_dir"))

	quotas, err := fetchQuotaReport("")
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
		err := createVastDirectoryQuota(abspath, DefaultUserQuotaLimit, DefaultUserFilesLimit)
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

func getDirectoryQuota() {
	dirPath := strings.TrimSuffix(*getQuotaPath, "/")
	if len(dirPath) == 0 {
		log.Fatalf("Please provide a directory path (--path)")
	}

	if !strings.HasPrefix(dirPath, "/") {
		log.Fatalf("Paths must be absolute: %s", *quotaPath)
	}

	quotas, err := fetchQuotaReport(dirPath)
	if err != nil {
		log.Fatalf("Failed to fetch quota report for %s: %s", dirPath, err)
	}

	fmt.Printf(LongFormat, "Path ", "files", "used", "soft", "hard", "grace ")
	for _, q := range quotas {
		fmt.Printf(LongFormat,
			q.Path,
			humanize.Comma(int64(q.UsedInodes)),
			humanize.Bytes(uint64(q.UsedEffectiveCapacity)),
			humanize.Bytes(uint64(q.SoftLimit)),
			humanize.Bytes(uint64(q.HardLimit)),
			q.GracePeriod)
	}
}

func cacheQuotas() {
	quotas, err := fetchQuotaReport("")
	if err != nil {
		log.Fatalf("Failed to fetch quota report from vast: %s", err)
	}

	log.Infof("Found %d quotas from vast", len(quotas))

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

		log.Infof("Successfully cached %s quota for %s", humanize.Bytes(uint64(q.SoftLimit)), q.Path)
	}
}

func main() {
	viper.ReadInConfig()
	kingpin.HelpFlag.Short('h')
	cmd := kingpin.Parse()

	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})
	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	switch cmd {
	case cmdUserCheck.FullCommand():
		checkAndSetUserQuotas()

	case cmdSetQuota.FullCommand():
		setDirectoryQuota()

	case cmdGetQuota.FullCommand():
		getDirectoryQuota()

	case cmdCache.FullCommand():
		cacheQuotas()
	default:
		kingpin.Usage()
	}
}
