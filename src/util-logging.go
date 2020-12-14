package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

type myLoggingType struct {
	filePath string
}

var (
	lg myLoggingType
)

// SetLogPath function
func (l *myLoggingType) register(logPath string) {
	l.filePath = logPath

	f, e := os.Open(l.filePath)
	defer f.Close()

	if e != nil {
		e2 := ioutil.WriteFile(l.filePath, []byte(""), 0644)
		if e2 != nil {
			os.Exit(1)
		}
	}
}

// LogHTTP function
func (l *myLoggingType) http(httpStatusCode int, r *http.Request, detail string) {
	out := time.Now().String() + ", "
	out += "[HTTP] " + r.Header.Get("X-Forwarded-For") + ", " + r.Method + " " + r.RequestURI + ", " +
		strconv.Itoa(httpStatusCode)
	if detail != "" {
		out += ", (" + detail + ")"
	}
	out += "\n"
	supportStr2FileAppend(l.filePath, out, 0644)
}

// Log function
func (l *myLoggingType) out(detail string) {
	out := time.Now().String() + ", "
	out += "[INFO] " + detail + "\n"
	supportStr2FileAppend(l.filePath, out, 0644)
}

// LogError function
func (l *myLoggingType) err(detail string, e error) {
	if e != nil {
		pc := make([]uintptr, 15)
		n := runtime.Callers(2, pc)
		frames := runtime.CallersFrames(pc[:n])
		frame, _ := frames.Next()
		out := time.Now().String() + ", "
		out += "[ERR ] " + frame.File + ":" + strconv.Itoa(frame.Line) + ", " + frame.Function + ", " + detail + ", " + e.Error() + "\n"
		supportStr2FileAppend(l.filePath, out, 0644)
	}
}
