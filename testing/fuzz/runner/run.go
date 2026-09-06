// Package runner implements --run-endpoints: replaying the lines in a
// generate file against the live server and recording, per line, the HTTP
// status code and response body size (not content - bodies can be
// hundreds of megabytes for real blobs). It halts after 3 consecutive
// timeouts/connection failures, since that likely means the server has
// crashed or hung.
package runner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Options controls a run.
type Options struct {
	InPath       string
	ResultsPath  string // "" = stdout
	Timeout      time.Duration
	MaxBodyBytes int64 // 0 = unlimited
}

// Result is one recorded outcome, keyed by the input file's line number.
type Result struct {
	LineNo   int
	Method   string
	URL      string
	Status   string // numeric HTTP status, "TIMEOUT", or "ERROR:<msg>"
	BodySize int64
	Elapsed  time.Duration
}

// Run executes the replay. It returns the results gathered before any
// halt condition, plus a bool indicating whether it halted early.
func Run(opt Options) (results []Result, halted bool, err error) {
	in, err := os.Open(opt.InPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening input file: %w", err)
	}
	defer in.Close()

	var out *os.File
	if opt.ResultsPath == "" {
		out = os.Stdout
	} else {
		out, err = os.Create(opt.ResultsPath)
		if err != nil {
			return nil, false, fmt.Errorf("creating results file: %w", err)
		}
		defer out.Close()
	}
	w := bufio.NewWriter(out)
	defer w.Flush()
	fmt.Fprintln(w, "line_number,method,status,body_size_bytes,elapsed_ms,url")

	client := &http.Client{Timeout: opt.Timeout}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	consecutiveFailures := 0
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		method, rawURL := parseLine(raw)

		req, rerr := http.NewRequest(method, rawURL, nil)
		if rerr != nil {
			res := Result{LineNo: lineNo, Method: method, URL: rawURL, Status: "BADREQUEST:" + rerr.Error()}
			results = append(results, res)
			writeResult(w, res)
			consecutiveFailures = 0
			continue
		}

		start := time.Now()
		resp, derr := client.Do(req)
		elapsed := time.Since(start)

		if derr != nil {
			if isTimeoutOrConnErr(derr) {
				consecutiveFailures++
				res := Result{LineNo: lineNo, Method: method, URL: rawURL, Status: "TIMEOUT", Elapsed: elapsed}
				results = append(results, res)
				writeResult(w, res)
				fmt.Fprintf(os.Stderr, "line %d: TIMEOUT/connection failure (%v) [%d consecutive]\n", lineNo, derr, consecutiveFailures)
				if consecutiveFailures >= 3 {
					fmt.Fprintf(os.Stderr, "halting: 3 consecutive timeouts/connection failures - server may have crashed (last line: %d)\n", lineNo)
					w.Flush()
					return results, true, nil
				}
				continue
			}
			consecutiveFailures = 0
			res := Result{LineNo: lineNo, Method: method, URL: rawURL, Status: "ERROR:" + derr.Error(), Elapsed: elapsed}
			results = append(results, res)
			writeResult(w, res)
			continue
		}

		consecutiveFailures = 0
		n := readBodySize(resp.Body, opt.MaxBodyBytes)
		resp.Body.Close()
		res := Result{LineNo: lineNo, Method: method, URL: rawURL, Status: strconv.Itoa(resp.StatusCode), BodySize: n, Elapsed: elapsed}
		results = append(results, res)
		writeResult(w, res)
	}
	if serr := scanner.Err(); serr != nil {
		return results, false, fmt.Errorf("reading input file: %w", serr)
	}
	return results, false, nil
}

func parseLine(line string) (method, url string) {
	if tab := strings.IndexByte(line, '\t'); tab >= 0 {
		return strings.TrimSpace(line[:tab]), strings.TrimSpace(line[tab+1:])
	}
	// Fall back to whitespace split for hand-edited files.
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		return fields[0], fields[1]
	}
	return "GET", line
}

func readBodySize(body io.Reader, maxBytes int64) int64 {
	if maxBytes > 0 {
		n, _ := io.Copy(io.Discard, io.LimitReader(body, maxBytes))
		// Drain the rest without counting further, so the connection can
		// be reused, but don't let it run away either.
		io.Copy(io.Discard, io.LimitReader(body, 10<<20))
		return n
	}
	n, _ := io.Copy(io.Discard, body)
	return n
}

func writeResult(w *bufio.Writer, r Result) {
	fmt.Fprintf(w, "%d,%s,%s,%d,%d,%s\n", r.LineNo, r.Method, r.Status, r.BodySize, r.Elapsed.Milliseconds(), r.URL)
	w.Flush() // flush per-line so progress is visible / survives a later crash
}

// isTimeoutOrConnErr reports whether err indicates the server timed out or
// is unreachable (connection refused/reset, DNS failure, etc.) - the
// class of error that, 3 times in a row, should halt the run because the
// server may have crashed.
func isTimeoutOrConnErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	for _, s := range []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"i/o timeout",
		"EOF",
		"connection timed out",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
