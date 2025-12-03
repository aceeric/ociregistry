package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// runTest runs the test. Using the filters, the driver gradually increases the number of
// goroutines pulling images until all sets of images by filter are being pulled concurrently,
// each set in its own goroutine. Then the goroutines are scaled down and the test is stopped.
func runTest(config Config) error {
	logFile, err := openOutputFile(config.logFile)
	if err != nil {
		return err
	}
	fmt.Fprintf(logFile, "%s\ttest driver begin\n", now())

	// each goroutine has its own counter
	counters := make([]atomic.Uint64, len(config.patterns))

	stopCh := initStopChans(len(config.patterns))

	// to stop the metrics tabulation goroutine
	tallyCh := make(chan bool, 1)

	// so the test goroutines can log concurrently
	logCh := make(chan string, 1000)

	// duration is how long each goroutine runs before the next goroutine is
	// started (scale up) or stopped (scale down)
	duration := time.Duration(config.iterationSeconds) * time.Second

	metricsFile, err := openOutputFile(config.metricsFile)
	if err != nil {
		return err
	}
	// start the metrics calculator
	go tallyStats(metricsFile, tallyCh, &counters, config.tallySeconds)

	// allows puller goroutines to log concurrently
	go logPullers(logCh, logFile)

	// scale up
	for i := 0; i < len(config.patterns); i++ {
		fmt.Fprintf(logFile, "%s\ttest driver start test #%d with filter %s\n", now(), i, config.patterns[i])
		go pullOnePattern(i, stopCh[i], logCh, config, &counters[i], config.patterns[i])
		time.Sleep(duration)
	}
	// scale down
	for i := len(config.patterns) - 1; i >= 0; i-- {
		fmt.Fprintf(logFile, "%s\ttest driver stop test #%d\n", now(), i)
		signalBoolChan(stopCh[i])
		if i == 0 {
			// no need to wait after stopping the last goroutine
			break
		}
		time.Sleep(duration)
	}
	// shut down the tally goroutine
	signalBoolChan(tallyCh)

	// shut down the puller logger goroutine
	signalStrChan(logCh)

	fmt.Fprintf(logFile, "%s\ttest driver exit\n", now())
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

// pullOnePattern uses the pattern in config to filter the images in the config. It then pulls (and
// optionally prunes) those images repeatedly until signalled on the passed channel. It increments a
// count of images pulled in the passed atomic counter which is also accessed by the tallyStats
// function in another goroutine.
func pullOnePattern(testNum int, stopChan chan bool, logCh chan string, config Config, counter *atomic.Uint64, pattern string) {
	logCh <- fmt.Sprintf("%s\ttest goroutine #%d starting\n", now(), testNum)

	// pruneClient is used by this goroutine to prune the ociregistry server
	var pruneClient *http.Client

	// puller is used to pull from the upstream
	var puller imgpull.Puller

	firstPull := true
	tmpTarfile := ""
	if !config.dryRun {
		td, _ := os.MkdirTemp("", "")
		defer os.RemoveAll(td)
		tmpTarfile = fmt.Sprintf("%s/tarfile.tar", td)
		if config.prune {
			pruneClient = makePruneClient()
		}
	}
	// arg parsing validated this regex so ignore the error
	re, _ := regexp.Compile(pattern)

	// make a copy of the image pull list so this goroutine can shuffle it between passes
	images := config.images
	for {
		for i := range images {
			select {
			case <-stopChan:
				logCh <- fmt.Sprintf("%s\ttest goroutine #%d stopping\n", now(), testNum)
				return
			default:
				fullImage := fmt.Sprintf("%s/%s/%s:%s", config.pullthroughURL, config.registryURL, config.images[i].Repository, config.images[i].Tags[0])
				if re.MatchString(fullImage) {
					if !config.dryRun {
						if firstPull {
							p, err := makePuller(fullImage)
							logCh <- fmt.Sprintf("%s\tgoroutine #%d error making puller for %s, the error was: %s\n", now(), testNum, fullImage, err)
							if err != nil {
								return
							}
							puller = p
							defer puller.Close()
							firstPull = false
						} else {
							puller.SetUrl(fullImage)
						}
						if err := puller.PullTar(tmpTarfile); err != nil {
							logCh <- fmt.Sprintf("%s\tgoroutine #%d error pulling %s, the error was: %s\n", now(), testNum, fullImage, err)
							return
						}
					} else {
						// just so we don't peg the processor
						time.Sleep(time.Millisecond * 100)
					}
					counter.Add(1)
				}
			}
		}
		if config.shuffle {
			shuffleInPlace(images)
		}
		if config.prune && !config.dryRun {
			if err := doPrune(pruneClient, config.pullthroughURL, pattern); err != nil {
				logCh <- fmt.Sprintf("%s\tgoroutine #%d error pruning pattern %s, the error was: %s\n", now(), testNum, pattern, err)
				return
			}
		}
	}
}

// makePuller makes a puller and returns it
func makePuller(fullImage string) (imgpull.Puller, error) {
	opts := imgpull.PullerOpts{
		Url:      fullImage,
		Scheme:   "http",
		OStype:   runtime.GOOS,
		ArchType: runtime.GOARCH,
	}
	return imgpull.NewPullerWith(opts)
}

// now makes the logging functions a bit more concise by returning the current
// time in YYYY-MM-DD HH:MM:SS format.
func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// makePruneClient makes an http.Client for pruning
func makePruneClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: t,
	}
}

// doPrune prunes images from the Ociregistry under test matching the passed pattern. This forces the next
// cycle of pulling any matching images to again go to the upstream registry which supports load testing
// pull through (vs pull cached.)
func doPrune(client *http.Client, pullthroughURL string, pattern string) error {
	// Build URL with query parameters
	baseURL := fmt.Sprintf("http://%s/cmd/prune", pullthroughURL)
	params := url.Values{}
	params.Add("type", "pattern")
	params.Add("expr", pattern)
	params.Add("dryRun", "false")
	params.Add("count", "-1")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequest(http.MethodDelete, fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}
	// debug
	//body, _ := io.ReadAll(resp.Body)
	//fmt.Printf("Received request body: %s\n", string(body))

	return nil
}

// logPullers listens on the passed channel for log events by the
// puller goroutines so that all the puller goroutines can log to the same
// logfile concurrently.
func logPullers(ch chan string, logFile *os.File) {
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
			elapsed := time.Since(lastTime).Seconds()
			totVals := int64(0)
			for i := range curVals {
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

// shuffleInPlace randomizes the order of the passed iterable in place
func shuffleInPlace[T any](input []T) {
	for i := len(input) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		input[i], input[j] = input[j], input[i]
	}
}
