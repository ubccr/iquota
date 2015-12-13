// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
)

func redisGet(key string) (*iquota.QuotaResponse, error) {
	conn, err := redis.Dial("tcp", viper.GetString("redis"))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Failed connecting to redis server")
		return nil, err
	}
	defer conn.Close()

	rawJson, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Info("Failed to fetch quota from cache")
		return nil, err
	}

	qr := &iquota.QuotaResponse{}
	err = json.Unmarshal([]byte(rawJson), qr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed to Unmarshal quota")
		return nil, err
	}

	return qr, nil
}

func redisSet(key string, qr *iquota.QuotaResponse, expire int) error {
	conn, err := redis.Dial("tcp", viper.GetString("redis"))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Failed connecting to redis server")
		return err
	}
	defer conn.Close()

	out, err := json.Marshal(qr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed marshal quota response")
		return err
	}

	_, err = conn.Do("SETEX", key, expire, out)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed to set cache")
		return err
	}

	return nil
}

func FetchGroupQuotaCache(path, group string) (*iquota.QuotaResponse, error) {
	_, err := redisGet(fmt.Sprintf("%s:GROUP-NEG:%s", path, group))
	if err == nil {
		return nil, &iquota.IsiError{Code: "AEC_NOT_FOUND", Message: "Group not found"}
	}
	return redisGet(fmt.Sprintf("%s:GROUP:%s", path, group))
}

func FetchUserQuotaCache(path, user string) (*iquota.QuotaResponse, error) {
	return redisGet(fmt.Sprintf("%s:USER:%s", path, user))
}

func SetGroupQuotaCache(path, group string, qr *iquota.QuotaResponse) error {
	return redisSet(fmt.Sprintf("%s:GROUP:%s", path, group), qr, viper.GetInt("cache_expire"))
}

func SetGroupNegCache(path, group string) error {
	return redisSet(fmt.Sprintf("%s:GROUP-NEG:%s", path, group), &iquota.QuotaResponse{}, viper.GetInt("neg_cache_expire"))
}

func SetUserQuotaCache(path, user string, qr *iquota.QuotaResponse) error {
	return redisSet(fmt.Sprintf("%s:USER:%s", path, user), qr, viper.GetInt("cache_expire"))
}
