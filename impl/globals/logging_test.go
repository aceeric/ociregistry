package globals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestXlatLogLevel(t *testing.T) {
	tests := []struct {
		strLevel string
		logLevel log.Level
	}{
		{"DEBUG", log.DebugLevel},
		{"INFO", log.InfoLevel},
		{"WARN", log.WarnLevel},
		{"TRACE", log.TraceLevel},
		{"ERROR", log.ErrorLevel},
		{"ANYTHING-ELSE", log.FatalLevel},
	}
	for _, lvlTest := range tests {
		if xlatLogLevel(lvlTest.strLevel) != lvlTest.logLevel {
			t.FailNow()
		}
	}
}

func TestFileLogging(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(td)
	logfile := filepath.Join(td, "logfile")
	ConfigureLogging("DEBUG", logfile)
	log.Debug("TEST")
	expectedText := "level=debug msg=TEST"
	content, err := os.ReadFile(logfile)
	if err != nil {
		t.FailNow()
	}
	if !strings.Contains(string(content), expectedText) {
		t.FailNow()
	}
}
