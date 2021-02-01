// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("port", 8080)
	viper.SetDefault("home_dir", "/home")
}

// Start web server
func RunServer() error {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	h, err := NewHandler()
	if err != nil {
		return err
	}

	h.SetupRoutes(e)

	s := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", viper.GetString("bind"), viper.GetInt("port")),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	certFile := viper.GetString("cert")
	keyFile := viper.GetString("key")
	if certFile != "" && keyFile != "" {
		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}

		s.TLSConfig = cfg
		s.TLSConfig.Certificates = make([]tls.Certificate, 1)
		s.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}

		log.Printf("Running on https://%s:%d%s", viper.GetString("bind"), viper.GetInt("port"), "/")
	} else {
		log.Warn("**WARNING*** SSL/TLS not enabled. HTTP communication will not be encrypted and vulnerable to snooping.")
		log.Printf("Running on http://%s:%d%s", viper.GetString("bind"), viper.GetInt("port"), "/")
	}

	return e.StartServer(s)
}
