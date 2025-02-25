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
	preloadImages := "B"
	prune := "C"
	pruneBefore := "C2"
	arch := "D"
	opsys := "E"
	concurrent := 123
	pullTimeout := 789
	cmdline := "server --config-path %s --image-path %s --log-level %s --port %s --load-images %s" +
		" --preload-images %s --arch %s --os %s --list-cache --prune %s --prune-before %s --dry-run" +
		" --version --always-pull-latest --concurrent %d --pull-timeout %d"
	cmdline = fmt.Sprintf(cmdline, cfgPath, imgPath, logLvl, port, loadImages,
		preloadImages, arch, opsys, prune, pruneBefore, concurrent, pullTimeout)
	os.Args = strings.Split(cmdline, " ")
	args := parseCmdline()
	if args.configPath != cfgPath ||
		args.imagePath != imgPath ||
		args.logLevel != logLvl ||
		args.port != port ||
		args.loadImages != loadImages ||
		args.preloadImages != preloadImages ||
		args.prune != prune ||
		args.pruneBefore != pruneBefore ||
		args.arch != arch ||
		args.os != opsys ||
		args.concurrent != concurrent ||
		args.pullTimeout != pullTimeout ||
		!args.dryRun ||
		!args.listCache ||
		!args.version ||
		!args.alwaysPullLatest {
		t.Fail()
	}
}
