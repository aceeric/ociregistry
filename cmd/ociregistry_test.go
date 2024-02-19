package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCmdline(t *testing.T) {
	cfgPath := "X"
	imgPath := "Y"
	logLvl := "Z"
	port := "1"
	loadImgs := "A"
	arch := "B"
	opsys := "C"
	cmdline := "server --config-path %s --image-path %s --log-level %s --port %s --load-images %s --arch %s --os %s"
	cmdline = fmt.Sprintf(cmdline, cfgPath, imgPath, logLvl, port, loadImgs, arch, opsys)
	foo := strings.Split(cmdline, " ")
	os.Args = foo
	args := parseCmdline()
	if args.arch != arch ||
		args.configPath != cfgPath ||
		args.imagePath != imgPath ||
		args.loadImages != loadImgs ||
		args.logLevel != logLvl ||
		args.os != opsys ||
		args.port != port {
		t.Fail()
	}
}
