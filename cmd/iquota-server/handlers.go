// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net/http"
	group "os/user"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ubccr/iquota"
)

type Handler struct {
	cache *iquota.Cache
}

func NewHandler() (*Handler, error) {
	return &Handler{cache: &iquota.Cache{}}, nil
}

func (h *Handler) SetupRoutes(e *echo.Echo) {
	e.GET("/quota", KerbAuthRequired(h.Quota)).Name = "quota"
}

func (h *Handler) Quota(c echo.Context) error {
	u := c.Get("user")
	if u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user")
	}
	user := u.(*User)
	log.Infof("User %s requesting quota", user.UID)

	path := c.QueryParam("path")
	if len(path) > 0 {
		quota, err := h.cache.GetDirectoryQuotaCache(path)
		if err != nil {
			if errors.Is(err, iquota.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, nil)
			}

			log.WithFields(log.Fields{
				"err":  err,
				"path": path,
			}).Error("Failed to fetch quota by path")

			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get quota")
		}

		return c.JSON(http.StatusOK, []*iquota.Quota{quota})
	}

	userFilter := c.QueryParam("user")
	if len(userFilter) > 0 {
		if userFilter != user.UID && !user.IsAdmin() {
			return echo.ErrUnauthorized
		}

		quota, err := h.cache.GetDirectoryQuotaCache(fmt.Sprintf("%s/%s", viper.GetString("home_dir"), userFilter))
		if err != nil {
			if errors.Is(err, iquota.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, nil)
			}

			log.WithFields(log.Fields{
				"err":        err,
				"userFilter": userFilter,
			}).Error("Failed to fetch quota with user filter")

			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get quota")
		}

		return c.JSON(http.StatusOK, []*iquota.Quota{quota})
	}

	groupFilter := c.QueryParam("group")
	if len(groupFilter) > 0 {
		_, err := group.LookupGroup(groupFilter)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, nil)
		}

		if !user.HasGroup(groupFilter) && !user.IsAdmin() {
			return echo.ErrUnauthorized
		}

		quotas, err := h.cache.SearchDirectoryQuotaCache(groupFilter)
		if err != nil {
			if errors.Is(err, iquota.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, nil)
			}

			log.WithFields(log.Fields{
				"err":         err,
				"groupFilter": groupFilter,
			}).Error("Failed to fetch quota with group filter")

			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get quota")
		}

		return c.JSON(http.StatusOK, quotas)
	}

	// Default to returning quota for user
	quota, err := h.cache.GetDirectoryQuotaCache(fmt.Sprintf("%s/%s", viper.GetString("home_dir"), user.UID))
	if err != nil {
		if errors.Is(err, iquota.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, nil)
		}

		log.WithFields(log.Fields{
			"err": err,
			"uid": user.UID,
		}).Error("Failed to fetch quota for user")

		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get quota")
	}

	return c.JSON(http.StatusOK, []*iquota.Quota{quota})
}
