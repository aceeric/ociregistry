package globals

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const msg_base = "echo server %s:%s status=%d latency=%s host=%s ip=%s"
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

// GetEchoLoggingFunc gets the distribution server logging function
func GetEchoLoggingFunc() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			// don't log the health check because it clutters the log and it is intended to
			// be used by Kubernetes anyway so doesn't need to be logged
			if req.RequestURI == "/health" {
				return nil
			}

			// digests clutter the logs so shorten them
			dgst := re.FindStringSubmatch(req.RequestURI)
			if len(dgst) == 2 {
				req.RequestURI = strings.Replace(req.RequestURI, dgst[1], dgst[1][:10], 1)
			}

			flds := make([]any, logFldcnt)
			flds[0] = req.Method
			flds[1] = req.RequestURI
			flds[2] = res.Status
			flds[3] = time.Since(start)
			flds[4] = req.Host
			flds[5] = c.RealIP()
			if log.GetLevel() >= log.DebugLevel {
				hdrs := make([]string, 0)
				for key, values := range c.Request().Header {
					hdrvals := strings.Join(values, ",")
					hdrs = append(hdrs, fmt.Sprintf("%s: %s", key, hdrvals))
				}
				flds[6] = fmt.Sprintf("[%s]", strings.Join(hdrs, "; "))
			}

			switch {
			case res.Status >= 500:
				log.Errorf(msg, flds...)
			case res.Status >= 400:
				log.Warnf(msg, flds...)
			case res.Status >= 300:
				log.Infof(msg, flds...)
			default:
				log.Infof(msg, flds...)
			}
			return nil
		}
	}
}
