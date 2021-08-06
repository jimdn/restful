package restful

import (
	"log"
	"os"
)

// Logger is an interface for logging
type Logger interface {
	Debugf(format string, v ...interface{})
	Debugln(v ...interface{})
	Warnf(format string, v ...interface{})
	Warnln(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
}

// Log is a global log handle
// you can reassign Log to your own Logger which
// contains the methods Logger interface contains
var Log Logger

// ExtLog is the default Logger
type ExtLog struct {
	Logger *log.Logger
}

// Debugf prints debug log
func (l *ExtLog) Debugf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// Debugln prints debug log with a newline appended
func (l *ExtLog) Debugln(v ...interface{}) {
	l.Logger.Println(v...)
}

// Warnf prints warn log
func (l *ExtLog) Warnf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// Warnln prints warn log with new line
func (l *ExtLog) Warnln(v ...interface{}) {
	l.Logger.Println(v...)
}

// Fatalf prints fatal log
func (l *ExtLog) Fatalf(format string, v ...interface{}) {
	l.Logger.Fatalf(format, v...)
}

// Fatalln prints fatal log with new line
func (l *ExtLog) Fatalln(v ...interface{}) {
	l.Logger.Fatalln(v...)
}

func init() {
	var el ExtLog
	el.Logger = log.New(os.Stderr, "", log.LstdFlags)
	Log = &el
}
