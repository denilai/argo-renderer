package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[37m"
	colorReset  = "\033[0m"
)

type CustomFormatter struct{}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	timestamp := entry.Time.Format("2006-01-02T15:04:05-07:00")
	b.WriteString(timestamp)
	b.WriteString(" ")

	levelText := strings.ToUpper(entry.Level.String())
	levelColor := getColorByLevel(entry.Level)
	fmt.Fprintf(b, "%s%s%s ", levelColor, levelText, colorReset)

	b.WriteString(entry.Message)

	if len(entry.Data) > 0 {

		keys := make([]string, 0, len(entry.Data))
		for k := range entry.Data {
			keys = append(keys, k)
		}

		for _, k := range keys {
			fmt.Fprintf(b, "%s%s=%v%s ", colorBlue, k, entry.Data[k], colorReset)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func getColorByLevel(level logrus.Level) string {
	switch level {
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return colorRed
	case logrus.WarnLevel:
		return colorYellow
	case logrus.InfoLevel:
		return colorGreen
	case logrus.DebugLevel, logrus.TraceLevel:
		return colorGray
	default:
		return colorReset
	}
}
