/*
Tensile web stress test tool

Mike Hughes 2014
intermernet AT gmail DOT com

LICENSE BSD 3 Clause

 ByteSize function (and bytesize.go) taken from http://golang.org/doc/progs/eff_bytesize.go
 Copyright the Go Authors.
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
	app     = "Tensile web stress test tool v"
	version = "0.1"
)

var (
	reqs, max, numCPU, maxCPU, numErr, maxErr int

	urlStr, flagErr string
	reqsError       = "ERROR: -requests (-r) must be greater than 0\n"
	maxError        = "ERROR: -concurrent (-c) must be greater than 0\n"
	maxErrError     = "ERROR: -maxerror (-e) must be greater than 0, or -1 for unlimited\n"
	urlError        = "ERROR: -url (-u) cannot be blank\n"
	schemeError     = "ERROR: unsupported protocol scheme %s\n"
	errLimError     = "ERROR: maximum error limit reached: %d\n"
	errTotalError   = "ERROR: total errors: %d\n"
	cpuWarn         = "NOTICE: -cpu=%d is greater than the number of CPUs on this system\n\tChanging -cpu to %d\n\n"
	cpuLTE0Warn     = "NOTICE: -cpu=%d is less than 1\n\tChanging -cpu to 1\n\n"
	maxGTreqsWarn   = "NOTICE: -concurrent=%d is greater than -requests\n\tChanging -concurrent to %d\n\n"

	wg sync.WaitGroup
)

func init() {
	maxCPU = runtime.NumCPU()
	flag.IntVar(&numCPU, "cpu", 1, "Number of CPUs")
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
func dispatcher(reqChan chan *http.Request, quit chan bool) {
	defer close(reqChan)
	for i := 0; i < reqs; i++ {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Println(err)
		}
		select {
		case <-quit:
			return
		default:
			req.Header.Add("User-Agent", app+version)
			reqChan <- req
		}
	}
}

// Worker Pool
func workerPool(reqChan chan *http.Request, respChan chan response, quit chan bool) {
	defer close(respChan)
	t := &http.Transport{}
	defer t.CloseIdleConnections()
	defer wg.Wait()
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
		log.Printf(errLimError, numErr)
		chk = true
	}
	return chk
}

// Consumer
func consumer(respChan chan response, quit chan bool) (int64, int64) {
	defer close(quit)
	var (
		conns, size int64
		prevStatus  int
	)
	for r := range respChan {
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
			rSize := r.ContentLength
			if rSize >= 0 {
				size += rSize
			}
			conns++
		}
		r.closeBody()
	}
	return conns, size
}

func checkFlags() {
	flag.Parse()
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
	if numCPU < 1 {
		fmt.Printf(cpuLTE0Warn, numCPU)
		numCPU = 1
	}
	if max > reqs {
		fmt.Printf(maxGTreqsWarn, max, reqs)
		max = reqs
	}
}

func main() {
	checkFlags()
	fmt.Printf("\n\t%s\n\n", app+version)
	runtime.GOMAXPROCS(numCPU)
	reqChan := make(chan *http.Request)
	respChan := make(chan response)
	quit := make(chan bool, max)
	fmt.Printf("Target URL:\t%s\nRequests:\t%d\nConcurrent:\t%d\nProcessors:\t%d\n\n", urlStr, reqs, max, numCPU)
	start := time.Now()
	go dispatcher(reqChan, quit)
	go workerPool(reqChan, respChan, quit)
	fmt.Printf("Waiting for replies...\n\n")
	conns, size := consumer(respChan, quit)
	if numErr > 0 {
		log.Printf(errTotalError, numErr)
	}
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
	sizeHuman := byteSize(float64(size))
	fmt.Printf("Replies:\t%d\nTotal size:\t%s\nTotal time:\t%s\nAverage time:\t%s\n\n", conns, sizeHuman, took, average)
}
