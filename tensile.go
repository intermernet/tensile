/*
Tensile web stress test tool

Mike Hughes 2014
intermernet AT gmail DOT com

LICENSE BSD 3 Clause
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"
)

const (
	app     = "\n\tTensile web stress test tool v%s\n\n"
	version = "0.1"
)

var (
	reqs   int
	max    int
	numCPU int
	maxCPU int

	urlStr string

	flagErr     string
	reqsError   string = "ERROR: -reqs must be greater than 0\n"
	maxError    string = "ERROR: -concurrent must be greater than 0\n"
	urlError    string = "ERROR: -url cannot be blank\n"
	schemeError string = "ERROR: unsupported protocol scheme %s\n"

	cpuWarn       string = "NOTICE: -cpu=%d is greater than the number of CPUs on this system\n\tChanging -cpu to %d\n\n"
	maxGTreqsWarn string = "NOTICE: -concurrent=%d is greater than -requests\n\tChanging -concurrent to %d\n\n"

	wg sync.WaitGroup
)

func init() {
	flag.StringVar(&urlStr, "url", "http://localhost/", "Target URL")
	flag.StringVar(&urlStr, "u", "http://localhost/", "Target URL (short flag)")
	flag.IntVar(&reqs, "requests", 50, "Total requests")
	flag.IntVar(&reqs, "r", 50, "Total requests (short flag)")
	flag.IntVar(&max, "concurrent", 5, "Maximum concurrent requests")
	flag.IntVar(&max, "c", 5, "Maximum concurrent requests (short flag)")
	maxCPU = runtime.NumCPU()
	flag.IntVar(&numCPU, "cpu", maxCPU, "Number of CPUs")
}

type Response struct {
	*http.Response
	err error
}

// Dispatcher
func dispatcher(reqChan chan *http.Request) {
	defer close(reqChan)
	for i := 0; i < reqs; i++ {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Println(err)
		}
		reqChan <- req
	}
}

// Worker Pool
func workerPool(reqChan chan *http.Request, respChan chan Response) {
	defer close(respChan)
	t := &http.Transport{}
	defer t.CloseIdleConnections()
	for i := 0; i < max; i++ {
		wg.Add(1)
		go worker(t, reqChan, respChan)
	}
	wg.Wait()
}

// Worker
func worker(t *http.Transport, reqChan chan *http.Request, respChan chan Response) {
	defer wg.Done()
	for req := range reqChan {
		resp, err := t.RoundTrip(req)
		r := Response{resp, err}
		respChan <- r
	}
}

// Consumer
func consumer(respChan chan Response) (int64, int64) {
	var (
		conns      int64
		size       int64
		prevStatus int
	)
	for r := range respChan {
		switch {
		case r.err != nil:
			log.Println(r.err)
		case r.StatusCode >= 400:
			if r.StatusCode != prevStatus {
				log.Printf("ERROR: %s\n", r.Status)
			}
			prevStatus = r.StatusCode
		default:
			size += r.ContentLength
			if err := r.Body.Close(); err != nil {
				log.Println(r.err)
			}
		}
		conns++
	}
	return conns, size
}

func main() {
	// Flag checks
	flag.Parse()
	fmt.Printf(app, version)
	if reqs <= 0 {
		flagErr += reqsError
	}
	if max <= 0 {
		flagErr += maxError
	}
	if urlStr == "" {
		flagErr += urlError
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		flagErr += err.Error()
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		flagErr += fmt.Sprintf(schemeError, u.Scheme)
	}
	if flagErr != "" {
		log.Fatal(fmt.Errorf("\n%s", flagErr))
	}
	if numCPU > maxCPU {
		fmt.Printf(cpuWarn, numCPU, maxCPU)
		numCPU = maxCPU
	}
	if max > reqs {
		fmt.Printf(maxGTreqsWarn, max, reqs)
		max = reqs
	}
	// Start
	runtime.GOMAXPROCS(numCPU)
	reqChan := make(chan *http.Request)
	respChan := make(chan Response)
	fmt.Printf("Sending %d requests to %s with %d concurrent workers using %d CPUs.\n\n", reqs, urlStr, max, numCPU)
	start := time.Now()
	go dispatcher(reqChan)
	go workerPool(reqChan, respChan)
	fmt.Println("Waiting for replies...\n")
	conns, size := consumer(respChan)
	// Calculate stats
	took := time.Since(start)
	tookNS := took.Nanoseconds()
	var averageNS int64
	if conns != 0 {
		averageNS = tookNS / conns
	}
	average, err := time.ParseDuration(fmt.Sprintf("%d", averageNS) + "ns")
	if err != nil {
		log.Println(err)
	}
	fmt.Printf("Connections:\t%d\nConcurrent:\t%d\nTotal size:\t%d bytes\nTotal time:\t%s\nAverage time:\t%s\n", conns, max, size, took, average)
}
