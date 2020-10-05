// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// Proxy server to handle requests to Isilon OneFS API for smart quota
// reporting
package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

func init() {
	viper.SetConfigName("iquota")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/iquota/")
	viper.SetDefault("enable_cache", false)
	viper.SetDefault("redis", ":6379")
	viper.SetDefault("cache_expire", 500)
	viper.SetDefault("neg_cache_expire", 86400)
}

func main() {
	app := cli.NewApp()
	app.Name = "iquota-server"
	app.Authors = []cli.Author{cli.Author{Name: "Andrew E. Bruno", Email: "aebruno2@buffalo.edu"}}
	app.Usage = "iquota-server"
	app.Version = "0.0.6"
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "conf,c", Usage: "Path to conf file"},
		&cli.BoolFlag{Name: "debug,d", Usage: "Print debug messages"},
	}
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.InfoLevel)
		} else {
			logrus.SetLevel(logrus.WarnLevel)
		}

		conf := c.GlobalString("conf")
		if len(conf) > 0 {
			viper.SetConfigFile(conf)
		}

		err := viper.ReadInConfig()
		if err != nil {
			return fmt.Errorf("Failed reading config file - %s", err)
		}

		return nil
	}
	app.Action = func(c *cli.Context) {
		Server()
	}

	app.RunAndExitOnError()
}
