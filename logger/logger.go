// logsingleton/logger.go
package logger

import (
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"sharegap.net/nostrodomo/config"
)

var (
    once   sync.Once
    logger *logrus.Logger
)

func InitLogger(conf config.LoggingConfig) {
    once.Do(func() {
        logger = logrus.New()

        // Set the log level
        level, err := logrus.ParseLevel(conf.Level)
        if err != nil {
            level = logrus.InfoLevel
        }
        logger.SetLevel(level)

        // Set the log format
        switch conf.Format {
        case "json":
            logger.SetFormatter(&logrus.JSONFormatter{})
        case "text":
            logger.SetFormatter(&logrus.TextFormatter{})
        default:
            logger.SetFormatter(&logrus.TextFormatter{})
        }

        // Set the log output
        switch conf.Output {
        case "stdout":
            logger.SetOutput(os.Stdout)
        case "stderr":
            logger.SetOutput(os.Stderr)
        case "file":
            file, err := os.OpenFile(conf.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
            if err == nil {
                logger.SetOutput(file)
            } else {
                logger.Warn("Failed to log to file, using default stderr")
                logger.SetOutput(os.Stderr)
            }
        default:
            logger.SetOutput(os.Stderr)
        }
    })
}

func GetLogger() *logrus.Logger {
    if logger == nil {
        panic("Logger is not initialized. Call InitLogger first.")
    }
    return logger
}

func Info(args ...interface {}) {
	logger.Info(args...)
}

func Warn(args ...interface {}) {
	logger.Warn(args...)
}

func Error(args ...interface {}) {
	logger.Error(args...)
}

func Fatal(args ...interface {}) {
	logger.Fatal(args...)
}

func Panic(args ...interface {}) {
	logger.Panic(args...)
}

func Debug(args ...interface {}) {
	logger.Debug(args...)
}
