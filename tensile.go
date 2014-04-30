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

const version = "0.1"

var (
	urlStr string

	reqs int
	max  int

	wg     sync.WaitGroup
	numCPU int
	maxCPU int

	reqsError   string = "ERROR: -reqs must be greater than 0\n"
	maxError    string = "ERROR: -concurrent must be greater than 0\n"
	urlError    string = "ERROR: URL cannot be blank\n"
	schemeError string = "ERROR: unsupported protocol scheme %s\n"
	cpuError    string = "ERROR: -cpu cannot exceed %d on this system\n"
)

func init() {
	flag.StringVar(&urlStr, "url", "http://localhost/", "Target URL")
	flag.IntVar(&reqs, "reqs", 50, "Total requests")
	flag.IntVar(&max, "concurrent", 5, "Maximum concurrent requests")
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
		conns int64
		size  int64
	)
	for r := range respChan {
		if r.err != nil {
			log.Println(r.err)
		} else {
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
	fmt.Printf("\n\tTensile web stress test tool v%s\n\n", version)
	flagErr := ""
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
	if numCPU > maxCPU {
		flagErr += fmt.Sprintf(cpuError, maxCPU)
	}
	if flagErr != "" {
		log.Fatal(fmt.Errorf("\n%s", flagErr))
	}
	if max > reqs {
		fmt.Println("NOTICE: Concurrent requests is greater than number of requests.")
		fmt.Println("\tChanging concurrent requests to number of requests\n")
		max = reqs
	}
	// Start
	runtime.GOMAXPROCS(runtime.NumCPU())
	reqChan := make(chan *http.Request)
	respChan := make(chan Response)
	fmt.Printf("Sending %d requests to %s with %d concurrent workers.\n\n", reqs, urlStr, max)
	start := time.Now()
	go dispatcher(reqChan)
	go workerPool(reqChan, respChan)
	fmt.Println("Waiting for replies...\n")
	conns, size := consumer(respChan)
	// Calculate stats
	took := time.Since(start)
	ns := took.Nanoseconds()
	var av int64
	if conns != 0 {
		av = ns / conns
	}
	average, err := time.ParseDuration(fmt.Sprintf("%d", av) + "ns")
	if err != nil {
		log.Println(err)
	}
	fmt.Printf("Connections:\t%d\nConcurrent:\t%d\nTotal size:\t%d bytes\nTotal time:\t%s\nAverage time:\t%s\n", conns, max, size, took, average)
}
