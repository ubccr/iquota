// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/kerby"
)

var (
	negotiateHeader       = "Negotiate"
	wwwAuthenticateHeader = "WWW-Authenticate"
	authorizationHeader   = "Authorization"
)

// Kerberos SPNEGO authentication
func KerbAuthRequired(app *Application, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authReq := strings.Split(r.Header.Get(authorizationHeader), " ")
		if len(authReq) != 2 || authReq[0] != negotiateHeader {
			w.Header().Set(wwwAuthenticateHeader, negotiateHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		os.Setenv("KRB5_KTNAME", viper.GetString("keytab"))

		ks := new(kerby.KerbServer)
		err := ks.Init("")
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("KerbServer Init Error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer ks.Clean()

		err = ks.Step(authReq[1])
		w.Header().Set(wwwAuthenticateHeader, negotiateHeader+" "+ks.Response())

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("KerbServer Step Error")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		princ := ks.UserName()
		parts := strings.SplitN(princ, "@", 2)
		user := &User{Uid: parts[0]}

		user.Groups, err = FetchGroups(user.Uid)
		if err != nil {
			logrus.Error("Failed to fetch groups for user: %s", user.Uid)
		}

		context.Set(r, "user", user)
		next.ServeHTTP(w, r)
	})
}
