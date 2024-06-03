// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"os/user"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	munge "github.com/ubccr/gomunge"
)

// MUNGE authentication
func MungeAuthRequired(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authReq := c.Request().Header.Get(echo.HeaderAuthorization)
		if len(authReq) == 0 {
			return echo.ErrUnauthorized
		}

		cred, err := munge.Decode(authReq)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("Failed to decode munge cred")
			return echo.ErrUnauthorized
		}

		ouser, err := user.LookupId(cred.UidString())
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("Failed to find user account")
			return echo.ErrUnauthorized
		}

		u := &User{UID: ouser.Username}

		u.Groups, err = FetchGroups(u.UID)
		if err != nil {
			logrus.Errorf("Failed to fetch groups for user: %s", u.UID)
		}

		c.Set("user", u)
		return next(c)
	}
}
