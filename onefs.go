// Copyright 2015 iquota Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// Package iquota provides a Go client for the Isilon OneFS API and command
// line tools for reporting SmartQuotas
package iquota

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
)

const (
	SERVICE_PLATFORM  = "platform"
	SERVICE_NAMESPACE = "namespace"
	RESOURCE_SESSION  = "/session/1/session"
)

var (
	isiSessionPattern = regexp.MustCompile(`^isisessid=([0-9a-zA-Z\-]+);`)
)

// OneFS Client
type Client struct {
	host    string
	port    int
	session string
	user    string
	passwd  string

	certPool *x509.CertPool
}

// OneFS API error
type IsiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *IsiError) Error() string {
	return fmt.Sprintf("OneFS API Error: %s - %s", e.Code, e.Message)
}

// Create new OneFS api client
func NewClient(host string, port int, user, passwd, cacert string) *Client {
	c := &Client{host: host, port: port, user: user, passwd: passwd}
	if c.port == 0 {
		c.port = 8080
	}

	if len(c.host) == 0 {
		c.host = "localhost"
	}

	pem, err := ioutil.ReadFile(cacert)
	if err == nil {
		c.certPool = x509.NewCertPool()
		if !c.certPool.AppendCertsFromPEM(pem) {
			c.certPool = nil
		}
	}

	return c
}

// Return URL for OneFS api given a resource
func (c *Client) Url(resource string) string {
	return fmt.Sprintf("https://%s:%d%s", c.host, c.port, resource)
}

// Return new http client
func (c *Client) httpClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	//TLSClientConfig: &tls.Config{RootCAs: c.certPool}}
	client := &http.Client{Transport: tr}

	return client
}

// Return new http get request
func (c *Client) getRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if len(c.session) > 0 {
		req.Header.Set("Cookie", fmt.Sprintf("isisessid==%s", c.session))
	} else if len(c.user) > 0 && len(c.passwd) > 0 {
		req.SetBasicAuth(c.user, c.passwd)
	}

	return req, nil
}

// Authenticate to OneFS API and create a session for multiple requests over a
// period of time.
func (c *Client) NewSession() (string, error) {
	apiUrl := c.Url(RESOURCE_SESSION)

	payload := map[string]interface{}{
		"username": c.user,
		"password": c.passwd,
		"services": []string{SERVICE_PLATFORM, SERVICE_NAMESPACE}}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")

	client := c.httpClient()

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 201 {
		return "", fmt.Errorf("OneFS login failed with HTTP status code: %d", res.StatusCode)
	}

	cookie := res.Header.Get("Set-Cookie")
	if len(cookie) == 0 {
		return "", errors.New("OneFS login failed emtpy set-cookie header")
	}

	session := ""
	matches := isiSessionPattern.FindStringSubmatch(cookie)
	if len(matches) == 2 {
		session = matches[1]
	}

	if len(session) == 0 {
		return "", errors.New("OneFS login failed invalid set-cookie header")
	}

	c.session = session

	return session, nil
}
