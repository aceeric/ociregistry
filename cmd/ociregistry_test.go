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
	loadImages := "A"
	preloadImages := "Z"
	arch := "B"
	opsys := "C"
	concurrent := 123
	pullTimeout := 789
	cmdline := "server --config-path %s --image-path %s --log-level %s --port %s --load-images %s" +
		" --preload-images %s --arch %s --os %s --list-cache --version --always-pull-latest --concurrent %d --pull-timeout %d"
	cmdline = fmt.Sprintf(cmdline, cfgPath, imgPath, logLvl, port, loadImages, preloadImages, arch, opsys, concurrent, pullTimeout)
	os.Args = strings.Split(cmdline, " ")
	args := parseCmdline()
	if args.arch != arch ||
		args.configPath != cfgPath ||
		args.imagePath != imgPath ||
		args.loadImages != loadImages ||
		args.preloadImages != preloadImages ||
		args.logLevel != logLvl ||
		args.os != opsys ||
		args.port != port ||
		!args.listCache ||
		!args.version ||
		!args.alwaysPullLatest ||
		args.concurrent != concurrent ||
		args.pullTimeout != pullTimeout {
		t.Fail()
	}
}
