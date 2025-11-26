package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

const (
	// tallyInterval is the interval in seconds between metrics computation. E.g. if 1000
	// events are counted in a 10-second interval, then the event rate is 100/sec.
	tallyInterval = 10
)

// testRun has all the test params. Including the filters and list of images. The filters
// are used to create concurrency. For example, an image list with 1000 images, and a way
// to filter those into 10 sets will allow the test to run 10 concurrent goroutines, each
// pulling 100 images continuously. So - the filters drive the concurrency.
type testRun struct {
	filters          []string
	images           []ImageInfo
	registry         string
	iterationSeconds int
	metricsFile      string
	logFile          string
	prune            bool
}

// runTests runs the test. Using the filters, the driver gradually increases the number of
// goroutines pulling images until all sets of images by filter are being pulled concurrently,
// each set in its own goroutine. Then the goroutines are scaled down and the test is stopped.
func runTests(tr testRun) {
	counters := make([]atomic.Uint64, len(tr.filters))
	ch := initStopChans(len(tr.filters))
	tallyCh := make(chan bool)
	duration := time.Duration(tr.iterationSeconds) * time.Second

	go tallyStats(tallyCh, &counters)

	// scale up
	for i := 0; i < len(tr.filters); i++ {
		// TODO LOG
		fmt.Printf("%s start test #%d with filter %s\n", time.Now().Format("2006-01-02 15:04:05"), i, tr.filters[i])
		go doTest(ch[i], tr, &counters[i])
		time.Sleep(duration)
	}
	// scale down
	for i := len(tr.filters) - 1; i >= 0; i-- {
		// TODO LOG
		fmt.Printf("%s stop test #%d\n", time.Now().Format("2006-01-02 15:04:05"), i)
		ch[i] <- true
		if i != 0 {
			// no need to wait after stopping the last test
			time.Sleep(duration)
		}
	}
	// shut down the tally channel
	tallyCh <- true
}

// doTest uses the filter in the testRun struct at ordinal position 'testnum' to filter the images
// in the testRun struct. It then pulls (and optionally prunes) those images until signalled on the
// passed channel. It maintains a count of images pulled in the passed atomic counter which is
// used by this goroutine AND the tallyStats goroutine.
func doTest(ch chan bool, tr testRun, counter *atomic.Uint64) {
	// TODO make a test dir to store the image tarfile
	for {
		for i := 0; i < len(tr.images); i++ {
			select {
			case <-ch:
				return
			default:
				// TODO BUILD THE IMAGE URL
				image := "docker.io/hello-world:latest"
				puller, err := imgpull.NewPullerWith(imgpull.NewPullerOpts(image))
				if err != nil {
					fmt.Println(err)
					return
				} else if err := puller.PullTar("./hello-world.tar"); err != nil {
					fmt.Println(err)
				}

				counter.Store(counter.Add(1))
			}
		}
		// TODO RANDOMIZE

		// TODO IF PRUNE
		//   PRUNE
	}
}

// tallyStats tallies the rate of pulls for all concurrent pullers.
func tallyStats(ch chan bool, counters *[]atomic.Uint64) {
	ticker := time.NewTicker(tallyInterval * time.Second)
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
			// TODO TO METRICS FILE
			fmt.Printf("%s\t%f\n", t.Format("2006-01-02 15:04:05"), rate)
			lastVals = curVals
			lastTime = time.Now()
		}
	}
}

// getCounters gets the current counter values for all pullers.
func getCounters(counters *[]atomic.Uint64) []int64 {
	curVals := make([]int64, len(*counters))
	for i := 0; i < len(*counters); i++ {
		counter := &(*counters)[i]
		curVals[i] = int64(counter.Load())
	}
	return curVals
}

// initStopChans initializes the channels used to stop the puller goroutines.
func initStopChans(count int) []chan bool {
	ch := make([]chan bool, count)
	for i := 0; i < len(ch); i++ {
		ch[i] = make(chan bool)
	}
	return ch
}
