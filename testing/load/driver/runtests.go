package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// testRun has all the test params. Including the filters and list of images. The filters
// are used to create concurrency. For example, an image list with 1000 images, and a way
// to filter those into 10 sets will allow the test to run 10 concurrent goroutines, each
// pulling 100 images continuously. So - the filters drive the concurrency.
type testRun struct {
	patterns         []string
	images           []imageInfo
	registryURL      string
	pullthroughURL   string
	iterationSeconds int
	tallySeconds     int
	metricsFile      string
	logFile          string
	prune            bool
	dryRun           bool
	shuffle          bool
}

// runTests runs the test. Using the filters, the driver gradually increases the number of
// goroutines pulling images until all sets of images by filter are being pulled concurrently,
// each set in its own goroutine. Then the goroutines are scaled down and the test is stopped.
func runTests(tr testRun) error {
	logFile, err := openOutputFile(tr.logFile)
	if err != nil {
		return err
	}
	fmt.Fprintf(logFile, "%s\ttest driver begin\n", time.Now().Format("2006-01-02 15:04:05"))

	counters := make([]atomic.Uint64, len(tr.patterns))
	stopCh := initStopChans(len(tr.patterns))
	tallyCh := make(chan bool, 1)
	logCh := make(chan string, 1000)
	duration := time.Duration(tr.iterationSeconds) * time.Second

	metricsFile, err := openOutputFile(tr.metricsFile)
	if err != nil {
		return err
	}
	go tallyStats(metricsFile, tallyCh, &counters, tr.tallySeconds)

	go logTestGoroutines(logCh, logFile)

	// scale up
	for i := 0; i < len(tr.patterns); i++ {
		fmt.Fprintf(logFile, "%s\ttest driver start test #%d with filter %s\n", time.Now().Format("2006-01-02 15:04:05"), i, tr.patterns[i])
		go doTest(i, stopCh[i], logCh, tr, &counters[i], tr.patterns[i])
		time.Sleep(duration)
	}
	// scale down
	for i := len(tr.patterns) - 1; i >= 0; i-- {
		fmt.Fprintf(logFile, "%s\ttest driver stop test #%d\n", time.Now().Format("2006-01-02 15:04:05"), i)
		signalBoolChan(stopCh[i])
		if i != 0 {
			// no need to wait after stopping the last goroutine
			time.Sleep(duration)
		}
	}
	// shut down the tally channel and the log channel
	signalBoolChan(tallyCh)
	signalStrChan(logCh)
	fmt.Fprintf(logFile, "%s\ttest driver exit\n", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// signalBoolChan sends true to the passed channel but won't block if there's no listener.
// If there is a listener, true is returned, else false.
func signalBoolChan(ch chan bool) bool {
	select {
	case ch <- true:
		return true
	default:
		// no receiver on the channel
		return false
	}
}

// signalBoolChan sends "EOF" to the passed channel but won't block if there's no listener.
// If there is a listener, true is returned, else false.
func signalStrChan(ch chan string) bool {
	select {
	case ch <- "EOF":
		return true
	default:
		// no receiver on the channel
		return false
	}
}

// doTest uses the passed pattern to filter the images in the testRun struct. It then pulls (and
// optionally prunes) those images until signalled on the passed channel. It increments a count of
// images pulled in the passed atomic counter which is used by this goroutine AND the tallyStats
// goroutine.
func doTest(testNum int, stopChan chan bool, logCh chan string, tr testRun, counter *atomic.Uint64, pattern string) {
	logCh <- fmt.Sprintf("%s\ttest goroutine #%d starting\n", time.Now().Format("2006-01-02 15:04:05"), testNum)
	tmpTarfile := ""
	if !tr.dryRun {
		td, _ := os.MkdirTemp("", "")
		defer os.RemoveAll(td)
		tmpTarfile = fmt.Sprintf("%s/tarfile.tar", td)
	}
	// arg parsing validated this so ignore the error
	re, _ := regexp.Compile(pattern)

	// make a copy of the image pull list so this goroutine can shuffle it between passes
	images := tr.images
	for {
		for i := 0; i < len(images); i++ {
			select {
			case <-stopChan:
				logCh <- fmt.Sprintf("%s\ttest goroutine #%d stopping\n", time.Now().Format("2006-01-02 15:04:05"), testNum)
				return
			default:
				fullImage := fmt.Sprintf("%s/%s/%s:%s", tr.pullthroughURL, tr.registryURL, tr.images[i].Repository, tr.images[i].Tags[0])
				if re.MatchString(fullImage) {
					if !tr.dryRun {
						opts := imgpull.PullerOpts{
							Url:      fullImage,
							Scheme:   "http",
							OStype:   runtime.GOOS,
							ArchType: runtime.GOARCH,
						}
						puller, err := imgpull.NewPullerWith(opts)
						if err != nil {
							logCh <- fmt.Sprintf("%s\tgoroutine #%d error pulling %s, the error was: %s\n", time.Now().Format("2006-01-02 15:04:05"), testNum, fullImage, err)
							return
						} else if err := puller.PullTar(tmpTarfile); err != nil {
							logCh <- fmt.Sprintf("%s\tgoroutine #%d error pulling %s, the error was: %s\n", time.Now().Format("2006-01-02 15:04:05"), testNum, fullImage, err)
							return
						}
					} else {
						// just so we don't peg the processor
						time.Sleep(time.Millisecond * 100)
					}
					counter.Store(counter.Add(1))
				}
			}
		}
		if tr.shuffle {
			shuffleInPlace(images)
		}
		// TODO IF PRUNE
		//   PRUNE
	}
}

// logTestGoroutines listens on the passed channel for log events by the
// puller goroutines so that all the puller goroutines can log to the same
// logfile concurrently.
func logTestGoroutines(ch chan string, logFile *os.File) {
	for {
		select {
		case logMsg := <-ch:
			if logMsg == "EOF" {
				return
			}
			fmt.Fprint(logFile, logMsg)

		default:
			time.Sleep(time.Millisecond * 10)
		}
	}
}

// tallyStats tallies the rate of pulls for all concurrent pullers. Since only one
// go func is running to tally statistics, it can write directly to the metrics
// file. However it does contend with the puller goroutines for the atomic counters.
// When signalled on the passed channel, it returns.
func tallyStats(logFile *os.File, ch chan bool, counters *[]atomic.Uint64, tallySeconds int) {
	ticker := time.NewTicker(time.Second * time.Duration(tallySeconds))
	defer ticker.Stop()
	lastVals := getCounters(counters)
	lastTime := time.Now()
	for {
		select {
		case <-ch:
			return
		case t := <-ticker.C:
			curVals := getCounters(counters)
			elapsed := t.Sub(lastTime).Seconds()
			totVals := int64(0)
			for i := 0; i < len(curVals); i++ {
				totVals += curVals[i] - lastVals[i]
			}
			rate := float64(totVals) / elapsed
			fmt.Fprintf(logFile, "%s\t%f\t%d\n", t.Format("2006-01-02 15:04:05"), rate, runtime.NumGoroutine())
			lastVals = curVals
			lastTime = time.Now()
		}
	}
}

// getCounters gets the current counter values for all pullers. Each puller goroutine
// has its own atomic counter to minimize contention across puller goroutines.
func getCounters(counters *[]atomic.Uint64) []int64 {
	curVals := make([]int64, len(*counters))
	for i := 0; i < len(*counters); i++ {
		counter := &(*counters)[i]
		curVals[i] = int64(counter.Load())
	}
	return curVals
}

// initStopChans initializes the channels used to stop the puller goroutines. These
// are buffered channels of length one (each) because the puller goroutine won't be
// sitting waiting to be signalled, it will be pulling through the ociregistry server
// in a tight loop.
func initStopChans(count int) []chan bool {
	ch := make([]chan bool, count)
	for i := 0; i < len(ch); i++ {
		ch[i] = make(chan bool, 1)
	}
	return ch
}

// openOutputFile opens the passed file, expecting that it already exists
func openOutputFile(logPath string) (*os.File, error) {
	if logPath == "" {
		return os.Stdout, nil
	}
	if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644); err != nil {
		return nil, err
	} else {
		return f, nil
	}
}
