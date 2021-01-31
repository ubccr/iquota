// Copyright 2020 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package iquota

import (
	"encoding/json"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
)

type Cache struct {
	Expire int
}

func (c *Cache) redisSet(key string, iq *IQuota) error {
	conn, err := redis.Dial("tcp", ":6379")
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

func (c *Cache) SetDirectoryQuotaCache(path string, iq *IQuota) error {
	return c.redisSet(path, iq)
}
