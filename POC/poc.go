package main

import (
	"fmt"
	"sync"
	"time"
)

var images []string = []string{
	"docker.io/calico/apiserver:v3.27.0.tar",
	//"docker.io/calico/cni:v3.27.0.tar",
	//"docker.io/calico/kube-controllers:v3.27.0.tar",
	// "docker.io/calico/node:v3.27.0.tar",
	// "docker.io/calico/pod2daemon-flexvol:v3.27.0.tar",
	// "docker.io/calico/typha:v3.27.0.tar",
	// "docker.io/coredns/coredns:1.11.1.tar",
	// "docker.io/grafana/grafana:10.2.2.tar",
	// "docker.io/infoblox/dnstools:latest.tar",
	// "docker.io/kubernetesui/dashboard:v2.7.0.tar",
	// "docker.io/openebs/provisioner-localpv:3.5.0.tar",
}

var ps *pullSyncer = newPullSyncer()

var simulatedImageCache map[string]bool = map[string]bool{}

// this simulates multiple concurrent pulls on the registry API
func main() {
	for i := 0; i < 2; i++ {
		for _, image := range images {
			// manifest handler
			go handler(image)
		}
	}
	// give all the goroutines time to finish
	time.Sleep(100 * time.Second)
}

// simulate a handler .../manifests/ref
func handler(image string) {
	fmt.Printf("handler - called for image %s\n", image)
	// this simulates a file system check
	if simulatedImageCache[image] {
		fmt.Println("handler - already pulled: " + image)
		return
	}
	result := pullImage(image, 15000)
	fmt.Printf("handler - pull result for image %s = %t\n", image, result)
}

func pullImage(image string, waitMillis int) bool {
	// this needs to be replaced by a filesystem check
	if simulatedImageCache[image] {
		fmt.Println("pullImage - already pulled: " + image)
		return true
	}
	ch := make(chan bool)
	var result bool = false
	go doPull(image, ch)
	select {
	case result = <-ch:
		fmt.Printf("pullImage - successful pull: %s\n", image)
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		fmt.Printf("pullImage - error: time out waiting for pull: %s\n", image)
		result = false
	}
	close(ch)
	if result {
		simulatedImageCache[image] = true
	}
	fmt.Printf("pullImage - return from pullImage: %t\n", result)
	return result
}

func doPull(image string, ch chan bool) {
	// error this doesnt use the channel!
	if ps.enqueue(image, ch) {
		fmt.Printf("doPull - already enqueued: %s, added chan %v\n", image, ch)
		return
	}
	fmt.Printf("doPull - newly enqueued - calling crane pull: %s\n", image)
	theActualCranePullWrapper()
	fmt.Printf("doPull - back from crane pull: %s\n", image)
	ps.pullComplete(image)
}

func theActualCranePullWrapper() {
	time.Sleep(5 * time.Second)
}

type pullSyncer struct {
	mu      sync.Mutex
	pullMap map[string][]chan bool
}

func newPullSyncer() *pullSyncer {
	return &pullSyncer{
		pullMap: make(map[string][]chan bool),
	}
}

// if already enqueued, add channel and return true, else enqueue
// image and channel and return false
func (ps *pullSyncer) enqueue(image string, ch chan bool) bool {
	fmt.Printf("enqueue - begins for image: %s, chan: %v\n", image, ch)
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		fmt.Printf("enqueue - image already enqueued: %s - append chan %v\n", image, ch)
		ps.pullMap[image] = append(chans, ch)
	} else {
		fmt.Printf("enqueue image not enqueued: %s - enqueing with chan: %v\n", image, ch)
		ps.pullMap[image] = []chan bool{ch}
	}
	ps.mu.Unlock()
	return exists
}

// signal all waiters for image and remove image from queue
func (ps *pullSyncer) pullComplete(image string) {
	fmt.Printf("pullComplete - begins for image: %s\n", image)
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		fmt.Printf("pullComplete - signaling image: %s\n", image)
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					fmt.Printf("pullComplete write to closed channel for image: %s - ignore\n", image)
				}
			}()
			fmt.Printf("pullComplete - signal done for chan: %v\n", ch)
			ch <- true
		}
		fmt.Printf("pullComplete - remove image %s from map\n", image)
		delete(ps.pullMap, image)
	} else {
		fmt.Printf("pullComplete - not found image: %s\n", image)
	}
	ps.mu.Unlock()
}
