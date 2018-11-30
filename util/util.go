// Copyright 2017, Square, Inc.

package util

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/rs/xid"
	"github.com/satori/go.uuid"
)

// UUID generates a UUID without any "-" in it.
func UUID() string {
	return strings.Replace(uuid.Must(uuid.NewV4()).String(), "-", "", -1)
}

// XID generates a globally unique, 12-byte xid.
func XID() xid.ID {
	return xid.New()
}

// NewTLSConfig takes a cert, key, and ca file and creates a *tls.Config.
func NewTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tls.LoadX509KeyPair: %s", err)
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
