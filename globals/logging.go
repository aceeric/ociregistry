package globals

import (
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func SetLogLevel(level string) {
	log.SetLevel(xlatLogLevel(level))
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})
	log.SetReportCaller(true)
}

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

const msg = "echo server %s:%s status=%d latency=%s host=%s ip=%s"

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
