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
	reqs     int
	max      int
	numCPU   int
	maxCPU   int
	maxErr   int
	errCount int

	urlStr string

	flagErr     string
	reqsError   string = "ERROR: -requests must be greater than 0\n"
	maxError    string = "ERROR: -concurrent must be greater than 0\n"
	maxErrError string = "ERROR: -errorlimit must be greater than 0\n"
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
	flag.IntVar(&maxErr, "errorlimit", 1, "Maximum errors")
	flag.IntVar(&maxErr, "e", 1, "Maximum errors (short flag)")
	maxCPU = runtime.NumCPU()
	flag.IntVar(&numCPU, "cpu", maxCPU, "Number of CPUs")
}

type response struct {
	*http.Response
	err error
}

func processing(waitChan chan bool) {
	wg.Wait()
	waitChan <- true
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
func workerPool(reqChan chan *http.Request, respChan chan response, quit chan bool) {
	defer close(respChan)
	t := &http.Transport{}
	defer t.CloseIdleConnections()
	for i := 0; i < max; i++ {
		wg.Add(1)
		go worker(t, reqChan, respChan, quit)
	}
	waitChan := make(chan bool)
	go processing(waitChan)
	for {
		select {
		case <-waitChan:
			return
		case <-quit:
			return
		}
	}
}

// Worker
func worker(t *http.Transport, reqChan chan *http.Request, respChan chan response, quit chan bool) {
	defer wg.Done()
	for req := range reqChan {
		select {
		case <-quit:
			return
		default:
			resp, err := t.RoundTrip(req)
			r := response{resp, err}
			respChan <- r
		}
	}
}

// Consumer
func consumer(respChan chan response, quit chan bool) (int64, int64) {
	defer close(quit)
	var (
		conns int64
		size  int64
	)
	for r := range respChan {
		if r.err != nil || r.StatusCode >= 400 {
			if r.err != nil {
				log.Println(r.err)
			} else {
				log.Println(r.Status)
			}
			errCount++
		} else {
			size += r.ContentLength
		}
		if err := r.Body.Close(); err != nil {
			log.Println(r.err)
		}
		if errCount >= maxErr {
			for i := 0; i <= max; i++ {
				quit <- true
			}
			return conns, size
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
	if maxErr <= 0 {
		flagErr += maxErrError
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
	respChan := make(chan response)
	quit := make(chan bool)
	fmt.Printf("Sending %d requests to %s with %d concurrent workers using %d CPUs.\n\n", reqs, urlStr, max, numCPU)
	start := time.Now()
	go dispatcher(reqChan)
	go workerPool(reqChan, respChan, quit)
	fmt.Println("Waiting for replies...\n")
	conns, size := consumer(respChan, quit)
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
