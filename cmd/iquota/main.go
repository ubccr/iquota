// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// display quota usage and limits
package main

import (
	"crypto/x509"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

func init() {
	viper.SetConfigName("iquota")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/iquota/")

	viper.SetDefault("iquota_url", "http://localhost")
}

func main() {
	app := cli.NewApp()
	app.Name = "iquota"
	app.Authors = []cli.Author{cli.Author{Name: "Andrew E. Bruno", Email: "aebruno2@buffalo.edu"}}
	app.Usage = "displays CCR quotas"
	app.Version = "0.0.6"
	app.HideVersion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "conf,c", Usage: "Path to conf file"},
		&cli.BoolFlag{Name: "debug,d", Usage: "Print debug messages"},
		&cli.BoolFlag{Name: "user,u", Usage: "Print user quota"},
		&cli.BoolFlag{Name: "long,l", Usage: "display long listing"},
		&cli.StringFlag{Name: "show-user", Usage: "Print user quota for specified user (super-user only)"},
		&cli.StringFlag{Name: "show-group", Usage: "Print group quota for specified group"},
		&cli.StringFlag{Name: "p,path,f,filesystem", Usage: "report quota for filesystem path"},
	}
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.InfoLevel)
		} else {
			logrus.SetLevel(logrus.FatalLevel)
		}

		conf := c.GlobalString("conf")
		if len(conf) > 0 {
			viper.SetConfigFile(conf)
		}

		viper.ReadInConfig()

		return nil
	}
	app.Action = func(c *cli.Context) {
		client := &QuotaClient{
			Group:       c.Bool("group"),
			User:        c.Bool("user"),
			Long:        c.Bool("long"),
			UserFilter:  c.String("show-user"),
			GroupFilter: c.String("show-group"),
			Path:        c.String("path"),
		}

		cert := viper.GetString("iquota_cert")
		if len(cert) > 0 {
			pem, err := ioutil.ReadFile(cert)
			if err != nil {
				logrus.Fatal("Failed reading cacert file: ", err)
			}

			client.certPool = x509.NewCertPool()
			if !client.certPool.AppendCertsFromPEM(pem) {
				logrus.Fatal("Failed appending cacert file to pool: ", err)
			}
		}

		client.Run()
	}

	app.RunAndExitOnError()
}
