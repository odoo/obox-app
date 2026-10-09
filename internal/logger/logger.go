package logger

import (
	"obox-app/buildinfo"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log = logrus.New()

var logDir string

func InitLogger() {
	dir, err := os.UserConfigDir()
	if err != nil {
		Fatalf("Failed to get user config dir: %v", err)
	}
	logDir = filepath.Join(dir, "OboxApp", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		Fatalf("Failed to create log directory: %v", err)
	}

	filename := filepath.Join(logDir, "obox-app.log")

	log.SetOutput(&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    20, // MB
		MaxBackups: 5,  // keep last x files
		MaxAge:     5,  // days
		Compress:   false,
	})

	// log.SetReportCaller(true)

	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	log.SetLevel(logrus.InfoLevel)
	Infof("Starting Obox App v: %s", buildinfo.GetVersionInfo())
}

// Wrappers
func Info(args ...interface{})                  { log.Info(args...) }
func Infof(format string, args ...interface{})  { log.Infof(format, args...) }
func Warn(args ...interface{})                  { log.Warn(args...) }
func Warnf(format string, args ...interface{})  { log.Warnf(format, args...) }
func Error(args ...interface{})                 { log.Error(args...) }
func Errorf(format string, args ...interface{}) { log.Errorf(format, args...) }
func Fatal(args ...interface{})                 { log.Fatal(args...) }
func Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }
func Debug(args ...interface{})                 { log.Debug(args...) }
func Debugf(format string, args ...interface{}) { log.Debugf(format, args...) }

func SetDebugMode(enabled bool) {
	if enabled {
		log.SetLevel(logrus.DebugLevel)
		Infof("Debug mode enabled (Debug log level)")
	} else {
		log.SetLevel(logrus.InfoLevel)
		Infof("Debug mode disabled (Info log level)")
	}
}

func IsDebugMode() bool {
	return log.GetLevel() >= logrus.DebugLevel
}

func LogDirectory() string { return logDir }
