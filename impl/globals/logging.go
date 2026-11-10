package globals

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const msg_base = "server %s:%s status=%d latency=%s host=%s ip=%s"
const msg_debug = msg_base + " hdrs=%s"
const srch = `.*sha256:([a-f0-9]{64}).*`

var re = regexp.MustCompile(srch)
var msg = msg_base
var logFldcnt = 6

// ConfigureLogging sets logging attributes
func ConfigureLogging(level string, logfile string) error {
	log.SetLevel(xlatLogLevel(level))
	log.SetFormatter(&log.TextFormatter{})
	if logfile != "" {
		lf, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		log.SetOutput(lf)
	}
	if log.GetLevel() >= log.DebugLevel {
		msg = msg_debug
		logFldcnt = 7
	}
	return nil
}

// xlatLogLevel translates the passed 'level' string to a logger const
func xlatLogLevel(level string) log.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return log.DebugLevel
	case "INFO":
		return log.InfoLevel
	case "WARN":
		return log.WarnLevel
	case "ERROR":
		return log.ErrorLevel
	case "TRACE":
		return log.TraceLevel
	}
	return log.FatalLevel
}

// LoggingMiddleware logs each request with its status and latency. Server errors (5xx)
// are logged at error level and client errors (4xx) at warn level.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}

		next.ServeHTTP(sw, req)

		// don't log the health check because it clutters the log and it is intended to
		// be used by Kubernetes anyway so doesn't need to be logged
		if req.RequestURI == "/health" {
			return
		}

		// net/http sends 200 if the handler wrote nothing
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}

		// digests clutter the logs so shorten them
		uri := req.RequestURI
		dgst := re.FindStringSubmatch(uri)
		if len(dgst) == 2 {
			uri = strings.Replace(uri, dgst[1], dgst[1][:10], 1)
		}

		flds := make([]any, logFldcnt)
		flds[0] = req.Method
		flds[1] = uri
		flds[2] = status
		flds[3] = time.Since(start)
		flds[4] = req.Host
		flds[5] = remoteIP(req)
		if log.GetLevel() >= log.DebugLevel {
			hdrs := make([]string, 0)
			for key, values := range req.Header {
				hdrvals := strings.Join(values, ",")
				hdrs = append(hdrs, fmt.Sprintf("%s: %s", key, hdrvals))
			}
			flds[6] = fmt.Sprintf("[%s]", strings.Join(hdrs, "; "))
		}

		switch {
		case status >= 500:
			log.Errorf(msg, flds...)
		case status >= 400:
			log.Warnf(msg, flds...)
		default:
			log.Infof(msg, flds...)
		}
	})
}

// remoteIP returns the client address of the connection. Does not trust
// the client-controlled X-Forwarded-For or X-Real-IP headers.
func remoteIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

// statusWriter records the status code a handler sends, for logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// ReadFrom passes io.Copy through to the underlying connection, so blob downloads from a
// file can use the kernel's sendfile instead of copying through a buffer.
func (s *statusWriter) ReadFrom(src io.Reader) (int64, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	if rf, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(s.ResponseWriter, src)
}

// Flush supports streaming handlers that flush partial responses.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying ResponseWriter.
func (s *statusWriter) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
