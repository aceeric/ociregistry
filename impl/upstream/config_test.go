package upstream

import (
	"fmt"
	"testing"
)

var cfg = `
---
- name: %s
  description: %s
  auth:
    user: %s
    password: %s
  tls:
    ca: %s
    cert: %s
    key: %s`

func TestCfg(t *testing.T) {
	name := "foobar"
	description := "frobozz"
	user := "flathead"
	pass := "fizzbin"
	ca := "zorkmid"
	cert := "westlands"
	key := "eastlands"
	manifest := fmt.Sprintf(cfg, name, description, user, pass, ca, cert, key)
	parseConfig([]byte(manifest))
	entry, err := configEntryFor(name)
	if err != nil {
		t.Errorf(err.Error())
	}
	if entry.Description != description ||
		entry.Auth.User != user ||
		entry.Auth.Password != pass ||
		entry.Tls.CA != ca ||
		entry.Tls.Cert != cert ||
		entry.Tls.Key != key {
		t.Fail()
	}
}
