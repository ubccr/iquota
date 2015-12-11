// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// display quota usage and limits
package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/spf13/viper"
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
	app.Usage = "displays users' disk usage and limits.  By default only the user quotas are printed."
	app.Version = "0.0.1"
	app.HideVersion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "conf,c", Usage: "Path to conf file"},
		&cli.BoolFlag{Name: "debug,d", Usage: "Print debug messages"},
		&cli.BoolFlag{Name: "verbose,v", Usage: "will display quotas on filesystems where no storage is allocated"},
		&cli.BoolFlag{Name: "long,l", Usage: "display long listing"},
		&cli.BoolFlag{Name: "full-path", Usage: "show full path for nfs mounts"},
		&cli.BoolFlag{Name: "g", Usage: "Print group quotas for the group of which the user is a member"},
		&cli.BoolFlag{Name: "u", Usage: "Print user quota"},
		&cli.StringFlag{Name: "user", Usage: "Print user quota for specified user (super-user only)"},
		&cli.StringFlag{Name: "group", Usage: "Print group quotas for specified group(s)."},
		&cli.StringFlag{Name: "f,filesystem", Usage: "report quotas only for filesystems specified on command line"},
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
			Verbose:     c.Bool("verbose"),
			Group:       c.Bool("g"),
			User:        c.Bool("u"),
			Long:        c.Bool("long"),
			FullPath:    c.Bool("full-path"),
			UserFilter:  c.String("user"),
			GroupFilter: c.String("group"),
			Filesystem:  c.String("filesystem")}

		client.Run()
	}

	app.RunAndExitOnError()
}
