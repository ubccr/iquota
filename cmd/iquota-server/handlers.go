// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/context"
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
		if len(qp.User) != 0 {
			if !user.IsAdmin() {
				errorHandler(app, w, http.StatusBadRequest, &iquota.IsiError{Code: "AEC_BAD_REQUEST", Message: "Access denied"})
				return
			}

			uid = qp.User
		}

		c := NewOnefsClient()

		qres, err := c.FetchUserQuota(qp.Path, uid)
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

		c := NewOnefsClient()
		gquotas := make([]*iquota.Quota, 0)

		for _, group := range groups {
			qres, err := c.FetchGroupQuota(qp.Path, group)
			if err != nil {
				if ierr, ok := err.(*iquota.IsiError); ok {
					if ierr.Code == "AEC_NOT_FOUND" && len(qp.Group) == 0 {
						continue
					}
					logrus.WithFields(logrus.Fields{
						"err":   err.Error(),
						"group": group,
					}).Error("Failed to fetch group quota")
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
