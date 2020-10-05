// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
)

func errorHandler(app *Application, w http.ResponseWriter, status int, err *iquota.IsiError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err != nil {
		out, err := json.Marshal(err)
		if err != nil {
			logrus.Printf("Error encoding error message as json: %s", err)
			return
		}
		w.Write(out)
	}
}

func IndexHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.Get(r, "user").(*User)
		if user == nil {
			logrus.Error("index handler: user not found in request context")
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		quotas := make([]*iquota.Quota, 0)

		for _, q := range app.defaultUserQuota {
			quotas = append(quotas, q)
		}

		for _, q := range app.defaultGroupQuota {
			quotas = append(quotas, q)
		}

		out, err := json.Marshal(quotas)
		if err != nil {
			logrus.Printf("Error encoding data as json: %s", err)
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}

func UserQuotaHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.Get(r, "user").(*User)
		if user == nil {
			logrus.Error("user quota handler: user not found in request context")
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		qp := new(iquota.QuotaParams)
		app.decoder.Decode(qp, r.URL.Query())

		if len(qp.Path) == 0 {
			errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Path is required"})
			return
		}

		uid := user.Uid
		if len(qp.User) != 0 && qp.User != uid {
			if !user.IsAdmin() {
				errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Access denied"})
				return
			}

			uid = qp.User
		}

		var qres *iquota.QuotaResponse

		if viper.GetBool("enable_cache") {
			cqres, err := FetchUserQuotaCache(qp.Path, uid)
			if err == nil {
				qres = cqres
			}
		}

		if qres == nil {
			c := NewOnefsClient()
			res, err := c.FetchUserQuota(qp.Path, uid)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err": err.Error(),
					"uid": uid,
				}).Error("Failed to fetch user quota")
				if ierr, ok := err.(*iquota.IsiError); ok {
					errorHandler(app, w, http.StatusBadRequest, ierr)
				} else {
					errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Fatal system error"})
				}

				return
			}

			qres = res

			if viper.GetBool("enable_cache") {
				SetUserQuotaCache(qp.Path, uid, qres)
			}
		}

		qr := &iquota.QuotaRestResponse{Quotas: qres.Quotas}
		qr.Default, _ = app.defaultUserQuota[qp.Path]

		out, err := json.Marshal(qr)
		if err != nil {
			logrus.Printf("Error encoding data as json: %s", err)
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}

func GroupQuotaHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.Get(r, "user").(*User)
		if user == nil {
			logrus.Error("group quota handler: user not found in request context")
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		qp := new(iquota.QuotaParams)
		app.decoder.Decode(qp, r.URL.Query())

		if len(qp.Path) == 0 {
			errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Path is required"})
			return
		}

		groups := user.Groups
		if len(qp.Group) != 0 {
			if !user.IsAdmin() {
				errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Access denied"})
				return
			}

			groups = []string{qp.Group}
		}

		logrus.WithFields(logrus.Fields{
			"groups": groups,
			"user":   user.Uid,
		}).Info("User groups")

		c := NewOnefsClient()
		c.NewSession()
		gquotas := make([]*iquota.Quota, 0)

		for _, group := range groups {

			// Ensure group names don't contain spaces
			group = strings.Replace(group, " ", "", -1)

			var qres *iquota.QuotaResponse

			if viper.GetBool("enable_cache") {
				cqres, err := FetchGroupQuotaCache(qp.Path, group)
				if err != nil {
					if ierr, ok := err.(*iquota.IsiError); ok {
						if ierr.Code == "AEC_NOT_FOUND" && len(qp.Group) == 0 {
							continue
						}
						logrus.WithFields(logrus.Fields{
							"err":   ierr.Error(),
							"group": group,
						}).Error("Failed to fetch group quota")
						errorHandler(app, w, http.StatusBadRequest, ierr)
						return
					}
				}

				qres = cqres
			}

			if qres == nil {
				res, err := c.FetchGroupQuota(qp.Path, group)
				if err != nil {
					if ierr, ok := err.(*iquota.IsiError); ok {
						if ierr.Code == "AEC_NOT_FOUND" && viper.GetBool("enable_cache") {
							SetGroupNegCache(qp.Path, group)
						}

						if ierr.Code == "AEC_NOT_FOUND" && len(qp.Group) == 0 {
							continue
						}
						logrus.WithFields(logrus.Fields{
							"err":   ierr.Error(),
							"group": group,
						}).Error("Failed to fetch group quota with isi error")
						errorHandler(app, w, http.StatusBadRequest, ierr)
					} else {
						logrus.WithFields(logrus.Fields{
							"err":   err.Error(),
							"group": group,
						}).Error("Failed to fetch group quota")
						errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Fatal system error"})
					}

					return
				}

				qres = res
				if viper.GetBool("enable_cache") {
					SetGroupQuotaCache(qp.Path, group, qres)
				}
			}

			gquotas = append(gquotas, qres.Quotas...)
		}

		qr := &iquota.QuotaRestResponse{Quotas: gquotas}
		qr.Default, _ = app.defaultGroupQuota[qp.Path]

		out, err := json.Marshal(qr)
		if err != nil {
			logrus.Printf("Error encoding data as json: %s", err)
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}

func OverQuotaHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.Get(r, "user").(*User)
		if user == nil {
			logrus.Error("over quota handler: user not found in request context")
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		if !user.IsAdmin() {
			errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Access denied"})
			return
		}

		qp := new(iquota.QuotaParams)
		app.decoder.Decode(qp, r.URL.Query())

		c := NewOnefsClient()
		qres, err := c.FetchAllOverQuota(qp.Path)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("Failed to fetch over quota list")
			if ierr, ok := err.(*iquota.IsiError); ok {
				errorHandler(app, w, http.StatusBadRequest, ierr)
			} else {
				errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Fatal system error"})
			}

			return
		}

		qr := &iquota.QuotaRestResponse{Quotas: qres.Quotas}
		out, err := json.Marshal(qr)
		if err != nil {
			logrus.Printf("Error encoding data as json: %s", err)
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}

func AllQuotaHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.Get(r, "user").(*User)
		if user == nil {
			logrus.Error("all quota handler: user not found in request context")
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		if !user.IsAdmin() {
			errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Access denied"})
			return
		}

		qp := new(iquota.QuotaParams)
		app.decoder.Decode(qp, r.URL.Query())

		c := NewOnefsClient()
		qres, err := c.FetchQuota(qp.Path, qp.Type, "", true, false)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err.Error(),
			}).Error("Failed to all quotas")
			if ierr, ok := err.(*iquota.IsiError); ok {
				errorHandler(app, w, http.StatusBadRequest, ierr)
			} else {
				errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Fatal system error"})
			}

			return
		}

		qr := &iquota.QuotaRestResponse{Quotas: qres.Quotas}

		for {
			if len(qres.Resume) == 0 {
				break
			}
			qres, err = c.FetchQuotaResume(qres.Resume)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err": err.Error(),
				}).Error("Failed to fetch using resume")
				if ierr, ok := err.(*iquota.IsiError); ok {
					errorHandler(app, w, http.StatusBadRequest, ierr)
				} else {
					errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Fatal system error"})
				}

				return
			}

			qr.Quotas = append(qr.Quotas, qres.Quotas...)
		}

		if viper.GetBool("enable_cache") {
			// Include quotas from cache
			cqr, err := FetchAllQuotaCache(qp.Type)
			if err != nil {
				logrus.Errorf("Error fetching quotas from cache: %s", err)
				errorHandler(app, w, http.StatusInternalServerError, nil)
				return
			}

			logrus.Infof("Found %d quotas from cache", len(cqr.Quotas))
			qr.Quotas = append(qr.Quotas, cqr.Quotas...)
		}

		out, err := json.Marshal(qr)
		if err != nil {
			logrus.Printf("Error encoding data as json: %s", err)
			errorHandler(app, w, http.StatusInternalServerError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}
