package main

import (
	"./fastclient"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/pprof"
	"time"

	"./report"
	"github.com/valyala/fasthttp"
)

var (
	directory   = flag.String("dir", "/Users/gluo17/Documents/workspace/tokenizer/regression", "directory of email bodies")

	fileName = flag.String("r", "report.html", "Set filename to store final report")

	web      = flag.Bool("web", false, "Auto open generated report at browser")

	destinationUrl = flag.String("dest", "http://terraintrain.corp.ne1.yahoo.com:8080/api/v1/scan", "Auto open generated report at browser")

	// benchmark customizations
	sendHeaders = flag.Bool("sh", true, "Send only emails with usable headers")
	maxFileSize = flag.Int("maxfs", 50 * 1000 * 1000, "Cap on file size (kb)")
	minFileSize = flag.Int("minfs", 0, "Min file size (kb)")

	d = flag.Duration("d", 60*time.Second, "Cant be less than 20sec")
	t = flag.Duration("t", 100*time.Millisecond, "Request timeout")
	q = flag.Int("q", 0, "Request per second limit. Detect automatically, if not setted")
	c = flag.Int("c", 10, "Number of supposed clients")

	debug              = flag.Bool("debug", false, "Print debug messages if true")
	disableKeepAlive   = flag.Bool("k", false, "Disable keepalive if true")
	disableCompression = flag.Bool("disable-compression", false, "Disables compression if true")
	successStatusCode  = flag.Int("successStatusCode", fasthttp.StatusOK, "Status code on which a successful request would be determined")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

var usage = `Usage: fasthttploader [options...] <url>
Notice: fasthttploader would force aggressive burst stages before testing to detect max qps and number for clients.
To avoid this you need to set -c and -q parameters.
Options:
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *d < time.Second*20 {
		usageAndExit("Duration cant be less than 20s")
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	messagesRing := fastclient.NewMessagesRing(*destinationUrl,
		*directory, *maxFileSize, *minFileSize, *sendHeaders)

	run(*c, &messagesRing)

	if *web {
		err := report.OpenBrowser(*fileName)
		if err != nil {
			fmt.Printf("Can't open browser to display report: %s", err)
		}
	} else {
		command, err := report.PrintOpenBrowser(*fileName)
		if err != nil {
			fmt.Printf("Can't generate command to display report in browser: %s", err)
		}
		fmt.Printf("Check test results by executing next command:\n %s\n", command)
	}

	var hasHeaders int

	for _, detail := range messagesRing.UnderlyingData {
		if detail.BodyDetail.HasHeaders {
			hasHeaders++
		}
	}
	fmt.Printf("headerless=%d, headered=%d, avgBodySize=%gkb, maxBodySize=%gkb, minBodySize=%d\n\n", len(messagesRing.UnderlyingData) - hasHeaders,
		hasHeaders, messagesRing.AvgSize/1000.0, float64(messagesRing.MaxSz)/1000.0, messagesRing.MinSz)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}
}

var re = regexp.MustCompile("^([\\w-]+):\\s*(.+)")

func usageAndExit(msg string) {
	flag.Usage()
	if msg != "" {
		fmt.Print("----------------------------\nErr: ")
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	os.Exit(1)
}
