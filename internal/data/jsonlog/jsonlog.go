package jsonlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

type Level int8

const (
	LevelInfo    Level = iota // value of 0
	LevelWarning              // value of 1
	LevelError                // value of 2
	LevelFatal                // value of 3
	LevelOff                  // value of 4
)

type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

func New(out io.Writer, minlevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minlevel,
	}
}

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

func (l *Logger) Print(level Level, message string, properties map[string]string) (int, error) {
	if level < l.minLevel {
		return 0, nil
	}
	aux := struct {
		Level      string            `json:"level,omitempty"`
		Time       string            `json:"time,omitempty"`
		Message    string            `json:"message,omitempty"`
		Properties map[string]string `json:"properties,omitempty"`
		Trace      string            `json:"trace,omitempty"`
	}{
		Level:      level.String(),
		Time:       time.Now().Format(time.RFC3339),
		Message:    message,
		Properties: properties,
	}

	if level == LevelFatal {
		aux.Trace = string(debug.Stack())
	}

	buff := new(bytes.Buffer)
	err := json.NewEncoder(buff).Encode(aux)
	if err != nil {
		fmt.Println(LevelFatal.String() + ": couldn't encode the logs to json format: " + err.Error())
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.out.Write(buff.Bytes())
}

func (l *Logger) Info(message string, properties map[string]string) {
	l.Print(LevelInfo, message, properties)
}
func (l *Logger) Warn(message string, properties map[string]string) {
	l.Print(LevelInfo, message, properties)
}
func (l *Logger) Error(err error, properties map[string]string) {
	l.Print(LevelInfo, err.Error(), properties)
}
func (l *Logger) Fatal(err error, properties map[string]string) {
	l.Print(LevelInfo, err.Error(), properties)
	os.Exit(1)
}

func (l *Logger) Write(p []byte) (int, error) {
	return l.out.Write(p)
}
