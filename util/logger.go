package util

import (
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
)

// ConfigureLogger is called from the flag parser in order to pass log level and type on
func ConfigureLogger(logLevel *string) error {
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
		return errors.New("unknown loglevel")
	}

	return nil
}
