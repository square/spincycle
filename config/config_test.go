// Copyright 2017-2019, Square, Inc.

package config_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/v2/config"
)

func createTempFile(t *testing.T, content []byte) string {
	tmpfile, err := ioutil.TempFile("", "for_test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	return tmpfile.Name()
}

func TestLoadConfigFileNotExist(t *testing.T) {
	// Config file doesn't exist.
	err := config.Load("nonexistant_file.txt", nil)
	if !os.IsNotExist(err) {
		t.Errorf("expected a 'file does not exist' error, did not get one")
	}
}

func TestLoadConfigBadContent(t *testing.T) {
	// Config file exists, but contains bad content.
	content := []byte("%%---invalid_yaml")
	fileName := createTempFile(t, content)
	defer os.Remove(fileName)

	var actualConfig config.RequestManager
	err := config.Load(fileName, &actualConfig)
	if err == nil {
		t.Error("expected an error, did not get one")
	}
}

func TestLoadConfigRequestManager(t *testing.T) {
	// Valid Request Manager config file
	content := []byte(`
---
server:
  addr: ":8888"
mysql:
  dsn: root:@localhost:3306/request_manager_development
jr_client:
  url: "http://127.0.0.1:9999"
`)
	fileName := createTempFile(t, content)
	defer os.Remove(fileName)

	var actualConfig config.RequestManager
	err := config.Load(fileName, &actualConfig)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedConfig := config.RequestManager{
		Server: config.Server{
			Addr: ":8888",
		},
		MySQL: config.MySQL{
			DSN: "root:@localhost:3306/request_manager_development",
		},
		JRClient: config.HTTPClient{
			ServerURL: "http://127.0.0.1:9999",
		},
	}

	if diff := deep.Equal(actualConfig, expectedConfig); diff != nil {
		t.Error(diff)
	}
}

func TestLoadConfigJobRunner(t *testing.T) {
	// Valid Job Runner config file.
	content := []byte(`
---
server:
  addr: ":9999"
rm_client:
  url: "http://127.0.0.1:8888"
`)
	fileName := createTempFile(t, content)
	defer os.Remove(fileName)

	var actualConfig config.JobRunner
	err := config.Load(fileName, &actualConfig)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedConfig := config.JobRunner{
		Server: config.Server{
			Addr: ":9999",
		},
		RMClient: config.HTTPClient{
			ServerURL: "http://127.0.0.1:8888",
		},
	}

	if diff := deep.Equal(actualConfig, expectedConfig); diff != nil {
		t.Error(diff)
	}
}
