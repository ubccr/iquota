// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"

	"github.com/carbocation/interpose"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
)

type Application struct {
	decoder           *schema.Decoder
	defaultUserQuota  map[string]*iquota.Quota
	defaultGroupQuota map[string]*iquota.Quota
}

func NewApplication() (*Application, error) {
	app := &Application{}

	c := NewOnefsClient()

	qres, err := c.FetchQuota("", "default-user", "", false, false)
	if err != nil {
		return nil, err
	}

	app.defaultUserQuota = make(map[string]*iquota.Quota)
	for _, q := range qres.Quotas {
		app.defaultUserQuota[q.Path] = q
	}

	qres, err = c.FetchQuota("", "default-group", "", false, false)
	if err != nil {
		return nil, err
	}

	app.defaultGroupQuota = make(map[string]*iquota.Quota)
	for _, q := range qres.Quotas {
		app.defaultGroupQuota[q.Path] = q
	}

	app.decoder = schema.NewDecoder()

	return app, nil
}

func NewOnefsClient() *iquota.Client {
	return iquota.NewClient(viper.GetString("onefs_host"),
		viper.GetInt("onefs_port"),
		viper.GetString("onefs_user"),
		viper.GetString("onefs_pass"),
		viper.GetString("onefs_cert"))
}

func (a *Application) middlewareStruct() (*interpose.Middleware, error) {
	mw := interpose.New()
	mw.UseHandler(a.router())

	return mw, nil
}

func (a *Application) router() *mux.Router {
	router := mux.NewRouter()

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	router.Path("/").Handler(KerbAuthRequired(a, IndexHandler(a))).Methods("GET")
	router.Path("/quota").Handler(KerbAuthRequired(a, AllQuotaHandler(a))).Methods("GET")
	router.Path("/quota/user").Handler(KerbAuthRequired(a, UserQuotaHandler(a))).Methods("GET")
	router.Path("/quota/group").Handler(KerbAuthRequired(a, GroupQuotaHandler(a))).Methods("GET")
	router.Path("/quota/exceeded").Handler(KerbAuthRequired(a, OverQuotaHandler(a))).Methods("GET")

	return router
}

func init() {
	viper.SetDefault("port", 8080)
	viper.SetDefault("onefs_port", 8080)
	viper.SetDefault("onefs_host", "localhost")
}

func Server() {
	app, err := NewApplication()
	if err != nil {
		logrus.Fatal(err.Error())
	}

	middle, err := app.middlewareStruct()
	if err != nil {
		logrus.Fatal(err.Error())
	}

	http.Handle("/", middle)

	certFile := viper.GetString("cert")
	keyFile := viper.GetString("key")

	if certFile != "" && keyFile != "" {
		logrus.Printf("Listening on https://%s:%d", viper.GetString("bind"), viper.GetInt("port"))
		err := http.ListenAndServeTLS(fmt.Sprintf("%s:%d", viper.GetString("bind"), viper.GetInt("port")), certFile, keyFile, nil)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Fatal("Failed to run https server")
		}
	} else {
		logrus.Printf("Listening on http://%s:%d", viper.GetString("bind"), viper.GetInt("port"))
		logrus.Warn("**WARNING*** SSL/TLS not enabled. HTTP communication will not be encrypted and vulnerable to snooping.")
		err := http.ListenAndServe(fmt.Sprintf("%s:%d", viper.GetString("bind"), viper.GetInt("port")), nil)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Fatal("Failed to run http server")
		}
	}
}
