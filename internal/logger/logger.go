package logger

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
	once   sync.Once
)

func Init() {
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)
}

func Get() *logrus.Logger {
	once.Do(func() {
		if logger == nil {
			Init()
		}
	})
	return logger
}
