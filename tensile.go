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
	reqs, max, numCPU, maxCPU, numErr, maxErr int

	urlStr, flagErr string
	reqsError       string = "ERROR: -requests (-r) must be greater than 0\n"
	maxError        string = "ERROR: -concurrent (-c) must be greater than 0\n"
	maxErrError     string = "ERROR: -maxerror (-e) must be greater than 0, or -1 for unlimited\n"
	urlError        string = "ERROR: -url (-u) cannot be blank\n"
	schemeError     string = "ERROR: unsupported protocol scheme %s\n"
	ErrLimError     string = "ERROR: maximum error limit reached:\t%d\n"
	ErrTotalError   string = "ERROR: total errors:\t%d\n"
	cpuWarn         string = "NOTICE: -cpu=%d is greater than the number of CPUs on this system\n\tChanging -cpu to %d\n\n"
	maxGTreqsWarn   string = "NOTICE: -concurrent=%d is greater than -requests\n\tChanging -concurrent to %d\n\n"

	wg sync.WaitGroup
)

func init() {
	maxCPU = runtime.NumCPU()
	flag.IntVar(&numCPU, "cpu", maxCPU, "Number of CPUs")
	flag.IntVar(&reqs, "requests", 50, "Total requests")
	flag.IntVar(&reqs, "r", 50, "Total requests (short flag)")
	flag.IntVar(&max, "concurrent", 5, "Maximum concurrent requests")
	flag.IntVar(&max, "c", 5, "Maximum concurrent requests (short flag)")
	flag.IntVar(&maxErr, "maxerror", 1, "Maximum errors before exiting")
	flag.IntVar(&maxErr, "e", 1, "Maximum errors before exiting (short flag)")
	flag.StringVar(&urlStr, "url", "http://localhost/", "Target URL")
	flag.StringVar(&urlStr, "u", "http://localhost/", "Target URL (short flag)")
}

type response struct {
	*http.Response
	err error
}

// Close response Body
func (r *response) closeBody() {
	if err := r.Body.Close(); err != nil {
		log.Println(r.err)
	}
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
	defer wg.Wait()
	t := &http.Transport{}
	defer t.CloseIdleConnections()
	for i := 0; i < max; i++ {
		wg.Add(1)
		go worker(t, reqChan, respChan, quit)
	}
}

// Worker
func worker(t *http.Transport, reqChan chan *http.Request, respChan chan response, quit chan bool) {
	defer wg.Done()
	for {
		select {
		case req, ok := <-reqChan:
			if ok {
				resp, err := t.RoundTrip(req)
				respChan <- response{resp, err}
			} else {
				return
			}
		case <-quit:
			return
		}
	}
}

// Kill Workers
func killWorkers(quit chan bool) {
	for {
		select {
		case quit <- true:
		default:
			return
		}
	}
}

// Check maximum error count
func checkMaxErr(quit chan bool) bool {
	chk := false
	numErr++
	if numErr >= maxErr && maxErr != -1 {
		killWorkers(quit)
		log.Printf(ErrLimError, numErr)
		chk = true
	}
	return chk
}

// Consumer
func consumer(respChan chan response, quit chan bool) (int64, int64) {
	defer close(quit)
	defer func() {
		if numErr > 0 {
			log.Printf(ErrTotalError, numErr)
		}
	}()
	var (
		conns, size int64
		prevStatus  int
	)
	for r := range respChan {
		defer r.closeBody()
		switch {
		case r.err != nil:
			log.Println(r.err)
			if checkMaxErr(quit) {
				return conns, size
			}
		case r.StatusCode >= 400:
			if r.StatusCode != prevStatus {
				log.Printf("ERROR: %s\n", r.Status)
			}
			prevStatus = r.StatusCode
			if checkMaxErr(quit) {
				return conns, size
			}
		default:
			size += r.ContentLength
			conns++
		}
	}
	return conns, size
}

func main() {
	flag.Parse()
	fmt.Printf(app, version)
	// Flag Errors
	if reqs <= 0 {
		flagErr += reqsError
	}
	if max <= 0 {
		flagErr += maxError
	}
	if maxErr == 0 || maxErr < -1 {
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
	// Flag Warnings
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
	quit := make(chan bool, max)
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
