// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/godbus/dbus"
	"github.com/spf13/viper"
)

type User struct {
	UID    string   `json:"uid"`
	Groups []string `json:"groups"`
}

func (u *User) HasGroup(group string) bool {
	for _, g := range u.Groups {
		if g == group {
			return true
		}
	}

	return false
}

func (u *User) IsAdmin() bool {
	for _, x := range viper.GetStringSlice("admins") {
		if x == u.UID {
			return true
		} else if u.HasGroup(x) {
			return true
		}
	}

	return false
}

func FetchGroups(uid string) ([]string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	obj := conn.Object("org.freedesktop.sssd.infopipe", dbus.ObjectPath("/org/freedesktop/sssd/infopipe"))

	var groups []string
	err = obj.Call("org.freedesktop.sssd.infopipe.GetUserGroups", 0, uid).Store(&groups)
	if err != nil {
		return nil, err
	}

	return groups, nil
}
