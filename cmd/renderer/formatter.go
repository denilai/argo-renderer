// /werf-argo-renderer/cmd/renderer/formatter.go

package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// Цветовые ANSI-коды для терминала
const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[37m"
	colorReset  = "\033[0m"
)

// CustomFormatter реализует интерфейс logrus.Formatter
type CustomFormatter struct{}

// Format форматирует запись лога в нужный нам вид
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	// 1. Временная метка
	timestamp := entry.Time.Format("2006-01-02T15:04:05-07:00")
	b.WriteString(timestamp)
	b.WriteString(" ")

	// 2. Уровень логирования с цветом и без отступов
	levelText := strings.ToUpper(entry.Level.String())
	levelColor := getColorByLevel(entry.Level)
	fmt.Fprintf(b, "%s%s%s ", levelColor, levelText, colorReset)

	// 3. Сообщение
	b.WriteString(entry.Message)

	// 4. Поля (application=..., cmd=...)
	if len(entry.Data) > 0 {
		b.WriteString(" ") // Пробел перед полями
		keys := make([]string, 0, len(entry.Data))
		for k := range entry.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys) // Сортируем ключи для консистентного вывода
		for _, k := range keys {
			fmt.Fprintf(b, "%s%s=%v%s ", colorBlue, k, entry.Data[k], colorReset)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

// getColorByLevel возвращает цвет для уровня логирования
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
