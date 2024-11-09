package globals

import (
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const msg = "echo server %s:%s status=%d latency=%s host=%s ip=%s"
const srch = `.*sha256:([a-f0-9]{64}).*`

var re = regexp.MustCompile(srch)

// ConfigureLogging sets the logger level
func ConfigureLogging(level string) {
	log.SetLevel(xlatLogLevel(level))
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})
	//log.SetReportCaller(true)
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

			flds := make([]interface{}, 6)
			flds[0] = req.Method
			flds[1] = req.RequestURI
			flds[2] = res.Status
			flds[3] = time.Since(start)
			flds[4] = req.Host
			flds[5] = c.RealIP()

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
