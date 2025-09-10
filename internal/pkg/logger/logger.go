package logger

import "github.com/sirupsen/logrus"

var Log = logrus.New()

func InitLogger() {
	Log.SetLevel(logrus.InfoLevel)
}

func ParseLogLevel(levelStr string) logrus.Level {
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		Log.Warnf("Invalid log level '%s', defaulting to 'info'", levelStr)
		return logrus.InfoLevel
	}
	return level
}
