package restful

import (
	"log"
	"os"
)

type Logger interface {
	Debugf(format string, v ...interface{})
	Debugln(v ...interface{})
	Warnf(format string, v ...interface{})
	Warnln(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
}
// you can reassign Log to your own Logger which
// contains the methods above
var Log Logger

// default Logger
type ExtLog struct {
	Logger *log.Logger
}

func (l *ExtLog) Debugf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

func (l *ExtLog) Debugln(v ...interface{}) {
	l.Logger.Println(v...)
}

func (l *ExtLog) Warnf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

func (l *ExtLog) Warnln(v ...interface{}) {
	l.Logger.Println(v...)
}

func (l *ExtLog) Fatalf(format string, v ...interface{}) {
	l.Logger.Fatalf(format, v...)
}

func (l *ExtLog) Fatalln(v ...interface{}) {
	l.Logger.Fatalln(v...)
}

func init() {
	var el ExtLog
	el.Logger = log.New(os.Stderr, "" , log.LstdFlags)
	Log = &el
}
