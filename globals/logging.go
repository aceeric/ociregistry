package globals

import (
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var atom = zap.NewAtomicLevel()

func init() {
	InitLogging("info", false, "console")
}

func Logger() *zap.Logger {
	return logger
}

func LogLevel(level string) {
	atom.SetLevel(xlatLogLevel(level))
}

func InitLogging(level string, disabled bool, encoding string) {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var outputPaths []string = []string{"stderr"}
	var errorOutputPaths []string = []string{"stderr"}

	if disabled {
		outputPaths = []string{}
		errorOutputPaths = []string{}
	}

	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(xlatLogLevel(level)),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          encoding,
		EncoderConfig:     encoderCfg,
		OutputPaths:       outputPaths,
		ErrorOutputPaths:  errorOutputPaths,
		InitialFields:     map[string]interface{}{"pid": os.Getpid()},
	}
	logger = zap.Must(config.Build())
}

func xlatLogLevel(level string) zapcore.Level {
	var lvl zapcore.Level = zap.FatalLevel
	switch strings.ToUpper(level) {
	case "DEBUG":
		lvl = zap.DebugLevel
	case "INFO":
		lvl = zap.InfoLevel
	case "WARN":
		lvl = zap.WarnLevel
	case "ERROR":
		lvl = zap.ErrorLevel
	}
	return lvl
}

func EchoMiddleware(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
			}

			fields := []zapcore.Field{
				zap.Int("status", res.Status),
				zap.String("latency", time.Since(start).String()),
				zap.String("id", id),
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("host", req.Host),
				zap.String("remote_ip", c.RealIP()),
			}

			n := res.Status
			switch {
			case n >= 500:
				log.Error("Server error", fields...)
			case n >= 400:
				log.Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return nil
		}
	}
}
