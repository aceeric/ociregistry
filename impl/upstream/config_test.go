package upstream

import (
	"fmt"
	"os"
	"testing"
	"time"
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
	names := []string{"t1", "t2"}
	descriptions := []string{"t3", "t4"}
	users := []string{"t5", "t6"}
	passs := []string{"t7", "t8"}
	cas := []string{"t9", "t10"}
	certs := []string{"t11", "t12"}
	keys := []string{"t13", "t14"}

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fail()
	}
	f.Close()
	defer os.Remove(f.Name())

	// reload configuration every second
	go ConfigLoader(f.Name(), 1)

	for i := 0; i <= 1; i++ {
		name := names[i]
		description := descriptions[i]
		user := users[i]
		pass := passs[i]
		ca := cas[i]
		cert := certs[i]
		key := keys[i]
		manifest := fmt.Sprintf(cfg, name, description, user, pass, ca, cert, key)
		os.WriteFile(f.Name(), []byte(manifest), 0700)
		time.Sleep(time.Second * time.Duration(2))
		entry, err := configEntryFor(name)
		if err != nil {
			t.Errorf(err.Error())
		}
		if entry.Description != descriptions[i] ||
			entry.Auth.User != users[i] ||
			entry.Auth.Password != passs[i] ||
			entry.Tls.CA != cas[i] ||
			entry.Tls.Cert != certs[i] ||
			entry.Tls.Key != keys[i] {
			t.Fail()
		}
	}
}
