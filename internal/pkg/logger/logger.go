package logger

import "github.com/sirupsen/logrus"

var Log = logrus.New()

func InitLogger() {
	Log.SetLevel(logrus.InfoLevel)
}
