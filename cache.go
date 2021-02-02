// Copyright 2020 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package iquota

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	ErrNotFound = errors.New("not found")
)

type Quota struct {
	Path            string `json:"path"`
	GracePeriod     string `json:"pretty_grace_period"`
	HardLimit       int    `json:"hard_limit"`
	SoftLimit       int    `json:"soft_limit"`
	Used            int    `json:"used"`
	HardLimitInodes int    `json:"hard_limit_inodes"`
	SoftLimitInodes int    `json:"soft_limit_inodes"`
	UsedInodes      int    `json:"used_inodes"`
}

type Cache struct {
	Expire int
}

func redisDial() (redis.Conn, error) {
	conn, err := redis.Dial("tcp", viper.GetString("redis"))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Failed connecting to redis server")
		return nil, err
	}

	return conn, err
}

func (c *Cache) redisFind(pattern string) ([]*Quota, error) {
	conn, err := redisDial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if strings.HasPrefix(pattern, "grp-") {
		pattern = strings.TrimPrefix(pattern, "grp-")
	}

	keys, err := redis.Strings(conn.Do("KEYS", fmt.Sprintf("*%s", pattern)))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			return nil, ErrNotFound
		}

		logrus.WithFields(logrus.Fields{
			"err":  err.Error(),
			"keys": keys,
		}).Error("Failed to find keys")
		return nil, err
	}

	var quotas []*Quota

	for _, key := range keys {
		if len(pattern) > 0 && strings.HasPrefix(key, viper.GetString("home_dir")) {
			continue
		}

		quota, err := c.unmarshalQuota(conn, key)
		if err != nil {
			continue
		}
		quotas = append(quotas, quota)
	}

	return quotas, nil
}

func (c *Cache) unmarshalQuota(conn redis.Conn, key string) (*Quota, error) {
	rawJson, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			return nil, ErrNotFound
		}

		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed to fetch quota from cache")
		return nil, err
	}

	quota := &Quota{}
	err = json.Unmarshal([]byte(rawJson), quota)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed to Unmarshal quota")
		return nil, err
	}

	return quota, nil
}

func (c *Cache) redisGet(key string) (*Quota, error) {
	conn, err := redisDial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return c.unmarshalQuota(conn, key)
}

func (c *Cache) redisSet(key string, iq *Quota) error {
	conn, err := redisDial()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Failed connecting to redis server")
		return err
	}
	defer conn.Close()

	out, err := json.Marshal(iq)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed marshal quota response")
		return err
	}

	_, err = conn.Do("SETEX", key, c.Expire, out)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
			"key": key,
		}).Error("Failed to set cache")
		return err
	}

	return nil
}

func (c *Cache) SetDirectoryQuotaCache(path string, iq *Quota) error {
	return c.redisSet(path, iq)
}

func (c *Cache) GetDirectoryQuotaCache(path string) (*Quota, error) {
	c.redisFind("grp-ezurek")
	return c.redisGet(path)
}

func (c *Cache) SearchDirectoryQuotaCache(pattern string) ([]*Quota, error) {
	return c.redisFind(pattern)
}
