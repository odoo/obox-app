package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"obox-app/internal/testutil"

	"github.com/sirupsen/logrus"
)

func TestInitLogger(t *testing.T) {
	InitLogger()

	dir := LogDirectory()
	testutil.ExpectedTrue(t, dir != "", "Expected LogDirectory() to return non-empty directory")

	info, err := os.Stat(dir)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, info.IsDir(), "Expected log path to be a directory")

	logFile := filepath.Join(dir, "obox-app.log")
	_, err = os.Stat(logFile)
	testutil.ExpectedNoError(t, err)

	testutil.ExpectedContains(t, dir, "OboxApp")
	testutil.ExpectedContains(t, dir, "logs")
}

func TestLoggingWrappers(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetLevel(logrus.DebugLevel)

	Info("info test message")
	testutil.ExpectedContains(t, buf.String(), "info test message")
	buf.Reset()

	Infof("infof %s %d", "arg", 42)
	testutil.ExpectedContains(t, buf.String(), "infof arg 42")
	buf.Reset()

	Warn("warn test message")
	testutil.ExpectedContains(t, buf.String(), "warn test message")
	buf.Reset()

	Warnf("warnf %s", "arg")
	testutil.ExpectedContains(t, buf.String(), "warnf arg")
	buf.Reset()

	Error("error test message")
	testutil.ExpectedContains(t, buf.String(), "error test message")
	buf.Reset()

	Errorf("errorf %s", "arg")
	testutil.ExpectedContains(t, buf.String(), "errorf arg")
	buf.Reset()

	Debug("debug test message")
	testutil.ExpectedContains(t, buf.String(), "debug test message")
	buf.Reset()

	Debugf("debugf %s", "arg")
	testutil.ExpectedContains(t, buf.String(), "debugf arg")
	buf.Reset()
}
