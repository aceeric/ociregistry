package preload

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// resultType defines a type for the 'result' struct
type resultType int

const (
	SUCCESS resultType = iota
	ERROR
)

// The result struct holds the result of an image pull. The 'count' field is the
// number of images pulled for an image URL. Since a typical image pull will result
// in a fat manifest (list of images), as well as a single image for os and arch, a
// typical successful image pull will grab two objects from the upstream distribution
// server.
type result struct {
	rt    resultType
	count int
	err   error
}

var (
	// wg references all running 'loadImage' goroutines
	wg sync.WaitGroup
	// taskChan implements the concurrency throttle: limiting the number of concurrent
	// 'loadImage' goroutines
	taskChan chan bool
	// resultChan is used by the 'loadImage' goroutine to communicate the outcome of
	// an image pull
	resultChan chan result
	// receiverChan is used to terminate the 'resultReceiver' goroutine
	receiverChan = make(chan bool)
	// readerChan is used by the 'resultReceiver' goroutine to indicate that a loader
	// goroutine has encountered a fatal error and the entire image load operation
	// should stop
	readerChan = make(chan error)
)

// Preload loads the manifest and blob cache at the passed 'imagePath' location from
// the list of images enumerated in the file identified by the passed 'imageListFile'
// arg. If an image is already present in cache, it is skipped. Otherwise the image is
// pulled from the upstream using the upstream registry encoded into the file entry.
// Here is a example of what one entry in the file identified by the 'imageListFile'
// arg should look like. It's a standard repository URL. If you can 'docker pull' it,
// then it should be valid in the file.
//
//	'registry.k8s.io/metrics-server/metrics-server:v0.6.2'
//
// The platform architecture and OS args are used to select an image from a "fat" manifest
// that contains a list of images. IMPORTANT: each item in the list MUST begin with
// a remote registry ref - i.e. to the left of the first forward slash
func Preload(imageListFile string, imagePath string, platformArch string, platformOs string, pullTimeout int, concurrent int) error {
	start := time.Now()
	log.Infof("loading images from file: %s", imageListFile)
	taskChan = make(chan bool, concurrent)
	resultChan = make(chan result, concurrent)

	f, err := os.Open(imageListFile)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	itemcnt := 0
	go resultReceiver(&itemcnt)

	var loadError error
SCANNER:
	// read throug the file a line at a time - each line is an image URL
	for scanner.Scan() {
		line := strings.TrimSpace(string(scanner.Bytes()))
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		select {
		// concurrency throttle
		case taskChan <- true:
			log.Infof("start image loader for: %s", line)
			wg.Add(1)
			go loadImage(line, imagePath, platformArch, platformOs, pullTimeout, &wg)
		// fatal error in image loader - abort
		case loadError = <-readerChan:
			log.Debug("received STOP on readerChan")
			break SCANNER
		}
	}

	log.Debug("wait loaders")
	wg.Wait()

	log.Debug("before select")
	select {
	// in case an image load error occurs before the loop above completed,
	// clear the channel.
	case <-readerChan:
	default:
		break
	}

	log.Debug("done waiting for loaders - stop receiver")
	receiverChan <- true
	log.Infof("loaded %d images to the file system cache in %s", itemcnt, time.Since(start))
	if loadError == nil {
		log.Info("no errors encountered")
	} else {
		log.Errorf("image pull aborted with error: %s", loadError)
	}
	close(taskChan)
	close(resultChan)
	close(receiverChan)
	close(readerChan)
	return nil
}

// resultReceiver reads from the 'resultChan' channel which is written to by the 'loadImage'
// goroutines. It watches for errors and - if an image load returns a non-nil error - it is
// presumed to be fatal. In that case the function sends the error on the 'readerChan'
// channel. This goroutine is signaled on the 'receiverChan' channel to stop it. As each
// image load completes, and reports the number of images pulled, this function tallies that
// to the passed 'itemcnt' arg.
func resultReceiver(itemcnt *int) {
	log.Debug("result receiver start")
OUTER:
	for {
		select {
		case r := <-resultChan:
			*itemcnt += r.count
			if r.err != nil {
				log.Debugf("error: %s - signal readerChan", r.err)
				readerChan <- r.err
				log.Debug("after signal readerChan")
			}
		case <-receiverChan:
			log.Debug("receiver signaled on receiverChan")
			break OUTER
		}
	}
	log.Debug("result receiver exit")
}

// The loadImage goroutine is a simple wrapper around 'preloadOneImage' with some concurrency
// handling. The result of the pull is sent to the 'resultChan' channel.
func loadImage(imageUrl string, imagePath string, platformArch string, platformOs string, pullTimeout int, wg *sync.WaitGroup) {
	log.Debugf("enter load image: %s", imageUrl)
	defer wg.Done()
	cnt, err := preloadOneImage(imageUrl, imagePath, platformArch, platformOs, pullTimeout)
	resultChan <- newResult(cnt, err)
	// concurrency throttle - allow another  'loadImage' goroutine
	<-taskChan
	log.Debugf("leave load image: %s", imageUrl)
}

// newResult creates a 'result' struct from the passed args
func newResult(count int, err error) result {
	r := result{
		rt:    SUCCESS,
		count: count,
		err:   err,
	}
	if err != nil {
		r.rt = ERROR
	}
	return r
}
