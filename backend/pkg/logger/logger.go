package logger

import (
	"fmt"
	"time"
)

type LogType int

const (
	TypeInfo LogType = iota
	TypeOK
	TypeWarn
	TypeError
)

var typeNames = map[LogType]string{
	TypeInfo:  "[INFO]",
	TypeOK:    "[OK]",
	TypeWarn:  "[WARN]",
	TypeError: "[ERROR]",
}

var typeColors = map[LogType]string{
	TypeInfo:  "\033[36m",
	TypeOK:    "\033[32m",
	TypeWarn:  "\033[33m",
	TypeError: "\033[31m",
}

type Logger struct {
	mode     string
	colorful bool
}

var Global *Logger

func Init(mode string, colorful bool) {
	if mode != "debug" && mode != "release" {
		mode = "release"
	}
	Global = &Logger{mode: mode, colorful: colorful}
}

func (l *Logger) log(logType LogType, format string, args ...interface{}) {
	if l.mode == "release" && logType == TypeInfo {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	if l.colorful {
		color := typeColors[logType]
		reset := "\033[0m"
		fmt.Printf("%s %s%s%s %s\n", timestamp, color, typeNames[logType], reset, message)
	} else {
		fmt.Printf("%s %s %s\n", timestamp, typeNames[logType], message)
	}
}

func Info(format string, args ...interface{}) {
	Global.log(TypeInfo, format, args...)
}

func OK(format string, args ...interface{}) {
	Global.log(TypeOK, format, args...)
}

func Warn(format string, args ...interface{}) {
	Global.log(TypeWarn, format, args...)
}

func Error(format string, args ...interface{}) {
	Global.log(TypeError, format, args...)
}
