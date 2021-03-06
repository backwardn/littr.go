package log

import (
	"fmt"
	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"sync"
	"time"
)

type Level int8

const (
	PanicLevel Level = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

type Logger interface {
	// Add ctx value pairs
	WithContext(...interface{}) Logger
	New(c ...interface{}) Logger

	Debug(string)
	Info(string)
	Warn(string)
	Error(string)
	Crit(string)

	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Critf(string, ...interface{})

	Print(...interface{})

	middleware.LogFormatter
}

type Ctx map[string]interface{}

type logger struct {
	l   logrus.FieldLogger
	ctx Ctx
	m   sync.RWMutex
}

func Dev(lvl Level) Logger {
	l := logger{}

	logrus.SetFormatter(&logrus.TextFormatter{
		QuoteEmptyFields: true,
		FullTimestamp:    false,
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.Level(lvl))

	l.l = logrus.StandardLogger()
	return &l
}

func Prod() Logger {
	l := logger{}

	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.Level(WarnLevel))

	l.l = logrus.StandardLogger()
	return &l
}

func context(l *logger) logrus.Fields {
	l.m.RLock()
	defer l.m.RUnlock()

	c := make(logrus.Fields, len(l.ctx))
	for k, v := range l.ctx {
		c[k] = v
	}

	return c
}

func (l logger) New(c ...interface{}) Logger {
	return l.WithContext(c)
}

func (l *logger) WithContext(ctx ...interface{}) Logger {
	l.m.Lock()
	defer l.m.Unlock()
	if l.ctx == nil {
		l.ctx = make(Ctx, 0)
	}
	for _, c := range ctx {
		switch cc := c.(type) {
		case Ctx:
			for k, v := range cc {
				l.ctx[k] = v
			}
		}
	}

	return l
}

func (l *logger) Debug(msg string) {
	l.l.WithFields(context(l)).Debug(msg)
	l.ctx = nil
}

func (l *logger) Debugf(msg string, p ...interface{}) {
	l.l.WithFields(context(l)).Debug(fmt.Sprintf(msg, p...))
	l.ctx = nil
}

func (l *logger) Info(msg string) {
	l.l.WithFields(context(l)).Info(msg)
	l.ctx = nil
}

func (l *logger) Infof(msg string, p ...interface{}) {
	l.ctx = nil
	l.l.WithFields(context(l)).Info(fmt.Sprintf(msg, p...))
	l.ctx = nil
}

func (l *logger) Warn(msg string) {
	l.l.WithFields(context(l)).Warn(msg)
	l.ctx = nil
}

func (l *logger) Warnf(msg string, p ...interface{}) {
	l.l.WithFields(context(l)).Warn(fmt.Sprintf(msg, p...))
	l.ctx = nil
}

func (l *logger) Error(msg string) {
	l.l.WithFields(context(l)).Error(msg)
	l.ctx = nil
}

func (l *logger) Errorf(msg string, p ...interface{}) {
	l.l.WithFields(context(l)).Error(fmt.Sprintf(msg, p...))
	l.ctx = nil
}

func (l *logger) Crit(msg string) {
	l.l.WithFields(context(l)).Fatal(msg)
	l.ctx = nil
}

func (l *logger) Critf(msg string, p ...interface{}) {
	l.l.WithFields(context(l)).Fatal(fmt.Sprintf(msg, p...))
	l.ctx = nil
}

func (l *logger) Print(i ...interface{}) {
	if i == nil || len(i) != 1 {
		return
	}
	l.Infof(i[0].(string))
}

type log struct {
	c Ctx
	m sync.RWMutex
	l *logger
}

func (l *log) Write(status, bytes int, elapsed time.Duration) {
	l.m.Lock()
	defer l.m.Unlock()

	l.c["duration"] = elapsed
	l.c["length"] = bytes
	l.c["status"] = status

	st := "OK"
	fn := l.l.WithContext(l.c).Info
	if status >= 400 {
		st = "FAIL"
		fn = l.l.WithContext(l.c).Warn
	}
	fn(st)
}

func (l *log) Panic(v interface{}, stack []byte) {
	l.c["stack"] = stack
	l.c["v"] = v

	l.l.WithContext(l.c).Crit("")
}

func (l *logger) NewLogEntry(r *http.Request) middleware.LogEntry {
	ll := log{
		c: Ctx{
			"met":   r.Method,
			"host":  r.Host,
			"uri":   r.RequestURI,
			"proto": r.Proto,
			"https": false,
		},
		l: l,
	}
	reqID := middleware.GetReqID(r.Context())
	l.m.Lock()
	defer l.m.Unlock()
	if reqID != "" {
		ll.c["id"] = reqID
	}
	if r.TLS != nil {
		ll.c["https"] = true
	}
	return &ll
}
