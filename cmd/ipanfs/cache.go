// Copyright 2020 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"github.com/ubccr/iquota"
)

type Cache struct {
	expire int
}

func (c *Cache) redisSet(key string, qr *iquota.QuotaResponse, expire int) error {
	conn, err := redis.Dial("tcp", ":6379")
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

func (c *Cache) SetGroupQuotaCache(path, group string, qr *iquota.QuotaResponse) error {
	return c.redisSet(fmt.Sprintf("%s:GROUP:%s", path, group), qr, c.expire)
}

func (c *Cache) SetUserQuotaCache(path, user string, qr *iquota.QuotaResponse) error {
	return c.redisSet(fmt.Sprintf("%s:USER:%s", path, user), qr, c.expire)
}
