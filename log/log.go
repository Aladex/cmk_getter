package log

// Overriding the default logger with logrus logger
//
// Path: log/log.go

import (
	"cmk_getter/config"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type LogrusFormatter struct {
	logrus.TextFormatter
	LevelDesc []string
}

const messageTemplate = "%s %s %s\n"

// Formatter function for logrus
func (f *LogrusFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Format timestamp to RFC3339
	f.TimestampFormat = "2006-01-02 15:04:05"
	// Return []byte from messageTemplate
	return []byte(fmt.Sprintf(messageTemplate, entry.Time.Format(f.TimestampFormat), f.LevelDesc[entry.Level], entry.Message)), nil
}

// Logger is a global logger
var Logger = logrus.New()

// SetLogLevel sets the log level
func SetLogLevel(level string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		Logger.Fatal(err)
	}
	Logger.SetLevel(lvl)
	plainFormatter := new(LogrusFormatter)
	plainFormatter.LevelDesc = []string{"PANC", "FATL", "ERRO", "WARN", "INFO", "DEBG"}
	Logger.SetFormatter(plainFormatter)

}

func init() {
	// Set log level from config
	SetLogLevel(config.ConfigCmkGetter.LogLevel)
}

func GinrusLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		Logger.Infoln("HTTP Request", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	}
}
