// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/kerby"
)

var (
	negotiateHeader = "Negotiate"
)

// Kerberos SPNEGO authentication
func KerbAuthRequired(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authReq := strings.Split(c.Request().Header.Get(echo.HeaderAuthorization), " ")
		if len(authReq) != 2 || authReq[0] != negotiateHeader {
			c.Response().Header().Set(echo.HeaderWWWAuthenticate, negotiateHeader)
			return echo.ErrUnauthorized
		}

		os.Setenv("KRB5_KTNAME", viper.GetString("keytab"))

		ks := new(kerby.KerbServer)
		err := ks.Init("")
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("KerbServer Init Error")
			return echo.NewHTTPError(http.StatusInternalServerError, "Fatal error")
		}
		defer ks.Clean()

		err = ks.Step(authReq[1])
		c.Response().Header().Set(echo.HeaderWWWAuthenticate, negotiateHeader+" "+ks.Response())

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("KerbServer Step Error")
			return echo.ErrUnauthorized
		}

		princ := ks.UserName()
		parts := strings.SplitN(princ, "@", 2)
		user := &User{UID: parts[0]}

		user.Groups, err = FetchGroups(user.UID)
		if err != nil {
			logrus.Error("Failed to fetch groups for user: %s", user.UID)
		}

		c.Set("user", user)
		return next(c)
	}
}
