package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type ansiColor int
type ansiMode int
type colorFunc func(string) string

// LogType defines the log formatting
type LogType int

// LogTypeAuto decides between Human and Machine based on TTY attached
// LogTypeHuman logs optimized for human reading
// LogTypeHumanNoColor disables colors from LogTypeHuman
// LogTypeMachine logs optimized for machine reading
// LogTypeMachineColor enables colors for LogTypeMachine
const (
	ansiColorBlack   = ansiColor(30)
	ansiColorRed     = ansiColor(31)
	ansiColorGreen   = ansiColor(32)
	ansiColorYellow  = ansiColor(33)
	ansiColorBlue    = ansiColor(34)
	ansiColorMagenta = ansiColor(35)
	ansiColorCyan    = ansiColor(36)
	ansiColorWhite   = ansiColor(37)
	ansiColorNone    = ansiColor(0)

	ansiModeNormal = ansiMode(0)
	ansiModeBold   = ansiMode(1)

	LogTypeAuto         = LogType(0)
	LogTypeHuman        = LogType(1)
	LogTypeHumanNoColor = LogType(2)
	LogTypeMachine      = LogType(3)
	LogTypeMachineColor = LogType(4)
)

var (
	levelString = []string{
		"PANIC",
		"FATAL",
		"ERROR",
		"WARN",
		"INFO",
		"DEBUG",
	}
	alignedLevelString = []string{
		"PANIC",
		"FATAL",
		"ERROR",
		"WARN ",
		"INFO ",
		"DEBUG",
	}
)

func newColorFunc(mode ansiMode, color ansiColor) colorFunc {
	if color == ansiColorNone {
		return func(s string) string { return s }
	}

	formatString := fmt.Sprintf("\033[%d;%dm%%s\033[0;0m", mode, color)
	return func(s string) string {
		return fmt.Sprintf(formatString, s)
	}
}

func hasTTY(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// LogFormatter implements a custom log formatter for logrus
type LogFormatter struct {
	sync.Once

	color     []colorFunc // indexed by level
	highlight colorFunc
	lowlight  colorFunc

	jsonFieldsRegex     *regexp.Regexp
	coloredFieldReplace []byte
	doFormat            func(*logrus.Entry) ([]byte, error)

	TimestampFormat string
	LogAsType       LogType
	ForceColored    bool
}

func (f *LogFormatter) setup(entry *logrus.Entry) {
	if f.TimestampFormat == "" {
		f.TimestampFormat = "2006-01-02 15:04:05"
	}

	if len(f.color) != len(logrus.AllLevels) {
		f.color = make([]colorFunc, len(logrus.AllLevels))
	}

	switch f.LogAsType {
	case LogTypeHuman:
		f.doFormat = f.formatForHumans

	case LogTypeHumanNoColor:
		f.doFormat = f.formatForHumansNoColor

	case LogTypeMachine, LogTypeMachineColor:
		f.doFormat = f.formatForMachines

	default:
		if hasTTY(os.Stdout) {
			f.doFormat = f.formatForHumans
		} else {
			f.doFormat = f.formatForMachines
		}
	}

	f.color[logrus.PanicLevel] = newColorFunc(ansiModeBold, ansiColorRed)
	f.color[logrus.FatalLevel] = newColorFunc(ansiModeBold, ansiColorRed)
	f.color[logrus.ErrorLevel] = newColorFunc(ansiModeNormal, ansiColorRed)
	f.color[logrus.WarnLevel] = newColorFunc(ansiModeNormal, ansiColorYellow)
	f.color[logrus.InfoLevel] = newColorFunc(ansiModeBold, ansiColorGreen)
	f.color[logrus.DebugLevel] = newColorFunc(ansiModeNormal, ansiColorBlue)

	f.highlight = newColorFunc(ansiModeNormal, ansiColorCyan)
	f.lowlight = newColorFunc(ansiModeBold, ansiColorBlack)
	f.coloredFieldReplace = []byte("\"" + f.highlight("$1") + "\":")
	f.jsonFieldsRegex, _ = regexp.Compile("\"([^\\\"]+)\"\\:")
}

// Format a logrus log message
func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	f.Do(func() { f.setup(entry) })
	return f.doFormat(entry)
}

func (f *LogFormatter) formatForMachines(entry *logrus.Entry) ([]byte, error) {
	data := entry.Data

	data["log.time"] = entry.Time.Format(f.TimestampFormat)
	data["log.level"] = levelString[entry.Level]
	data["log.message"] = entry.Message

	fields, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}

	if f.LogAsType == LogTypeMachineColor {
		fields = f.jsonFieldsRegex.ReplaceAll(fields, f.coloredFieldReplace)
	}

	return append(fields, '\n'), nil
}

func (f *LogFormatter) formatForHumans(entry *logrus.Entry) ([]byte, error) {
	levelString := alignedLevelString[entry.Level]
	timestamp := entry.Time.Format(f.TimestampFormat)
	coloredFields := ""

	if len(entry.Data) > 0 {
		fields, err := json.MarshalIndent(entry.Data, "", "  ")
		if err != nil {
			return []byte{}, err
		}
		coloredFields = string(f.jsonFieldsRegex.ReplaceAll(fields, f.coloredFieldReplace))
	}

	b := entry.Buffer
	if b == nil {
		b = &bytes.Buffer{}
	}

	b.WriteString(fmt.Sprintf("%s %s %s %s\n",
		f.lowlight(timestamp),
		f.color[entry.Level](levelString),
		entry.Message,
		coloredFields))

	return b.Bytes(), nil
}

func (f *LogFormatter) formatForHumansNoColor(entry *logrus.Entry) ([]byte, error) {
	levelString := alignedLevelString[entry.Level]
	timestamp := entry.Time.Format(f.TimestampFormat)
	fields := ""

	if len(entry.Data) > 0 {
		fieldAsByte, err := json.MarshalIndent(entry.Data, "", "  ")
		if err != nil {
			return []byte{}, err
		}
		fields = string(fieldAsByte)
	}

	b := entry.Buffer
	if b == nil {
		b = &bytes.Buffer{}
	}

	b.WriteString(fmt.Sprintf("%s %s %s %s\n",
		timestamp,
		levelString,
		entry.Message,
		fields))

	return b.Bytes(), nil
}

// ConfigureLogger is called from the flag parser in order to pass log level and type on
func ConfigureLogger(logLevel *string, logType *string) error {
	formatter := &LogFormatter{}

	switch strings.ToLower(*logType) {
	case "human":
		formatter.LogAsType = LogTypeHuman
	case "human-color":
		formatter.LogAsType = LogTypeHumanNoColor
	case "machine":
		formatter.LogAsType = LogTypeMachine
	case "machine+color":
		formatter.LogAsType = LogTypeMachineColor
	default:
		formatter.LogAsType = LogTypeAuto
	}

	logrus.SetFormatter(formatter)

	switch strings.ToLower(*logLevel) {
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	default:
		return errors.New("Unknown loglevel")
	}

	return nil
}
