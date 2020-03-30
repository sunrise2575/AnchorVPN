package support;

import (
	"strings"
	"os/exec"
	"os"
	"strconv"
	"io"
	"io/ioutil"
	"bytes"
	"github.com/tidwall/gjson"
	"runtime"
	"time"
	"net/http"
)

var logpath string;

// SetLogPath function
func SetLogPath(logPath string) {
	logpath = logPath;
	f, e := os.Open(logpath);
	defer f.Close();
	if e != nil {
		e2 := ioutil.WriteFile(logpath, []byte(""), 0644);
		if e2 != nil { os.Exit(1); }
	}
}

// LogHTTP function
func LogHTTP(httpStatusCode int, r *http.Request, detail string) {
	out := time.Now().String() + ", "
	out += "[HTTP] " + r.Header.Get("X-Forwarded-For") + ", " + r.Method + " " + r.RequestURI + ", " +
		strconv.Itoa(httpStatusCode);
	if detail != "" {
		out += ", (" + detail + ")";
	}
	out += "\n"
	Str2FileAppend(logpath, out, 0644);
}

// Log function
func Log(detail string) {
	out := time.Now().String() + ", "
	out += "[INFO] " + detail + "\n";
	Str2FileAppend(logpath, out, 0644);
}

// LogError function
func LogError(detail string, e error) {
	if e != nil {
		pc := make([]uintptr, 15)
		n := runtime.Callers(2, pc);
		frames := runtime.CallersFrames(pc[:n]);
		frame, _ := frames.Next();
		out := time.Now().String() + ", "
		out += "[ERR ] " + frame.File + ":" + strconv.Itoa(frame.Line) + ", " + frame.Function + ", " + detail + ", " + e.Error() + "\n";
		Str2FileAppend(logpath, out, 0644);
	}
}

// File2Str function
func File2Str(path string) string {
	tmp, e := ioutil.ReadFile(path);
	LogError("No such file!: " + path + " Error msg: " + path, e);
	return strings.TrimSuffix(string(tmp), "\n");
}

// Str2File function
func Str2File(path, data string, permission uint32) error {
	return ioutil.WriteFile(path, []byte(data), os.FileMode(permission));
}


// Str2FileAppend function
func Str2FileAppend(path, data string, permission uint32) error {
	f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.FileMode(permission));
	if e != nil { return e; }
	defer f.Close();
	_, e = f.WriteString(data);
	if e != nil { return e; }
	return nil
}

// JSON function
func JSON(path, query string) string {
	return gjson.Get(File2Str(path), query).String();
}

// JSONNestedArray function
func JSONNestedArray(path, query string, result chan string, subQuery... string) {
	defer close(result);
	tmp := gjson.Get(File2Str(path), query);
	tmp.ForEach(func (key, value gjson.Result) bool {
		for _, q := range subQuery {
			a := gjson.Get(value.String(), q).String();
			result <- a;
		}
		return true;
	});
}

// JSONSimpleArray function
func JSONSimpleArray(input string, result chan string) {
	defer close(result);
	input = "{ \"temporary\" : " + input + "}";
	tmp := gjson.Get(input, "temporary");
	tmp.ForEach(func (key, value gjson.Result) bool {
		result <- value.String();
		return true;
	});
}

// Run function
func Run(command string) string {
	s := strings.Split(command, " ");
	out, e := exec.Command(s[0], s[1:]...).Output();
	LogError("Command finished with error.", e);
	return strings.TrimSuffix(string(out), "\n");
}

// RunPipe function
// This pipe program is not working on wg show | grep blah blah!
func RunPipe(input... string) string {
	clist := []*exec.Cmd{};
	for _, in := range input {
		s := strings.Split(in, " ");
		clist = append(clist, exec.Command(s[0], s[1:]...));
	}

	r := make([]*io.PipeReader, len(clist)-1);
	w := make([]*io.PipeWriter, len(clist)-1);

    for i := 0; i < len(clist)-1; i++ {
		r[i], w[i] = io.Pipe();
	}

    for i := 1; i < len(clist); i++ {
		clist[i-1].Stdout = w[i-1];
		clist[i].Stdin = r[i-1];
	}

	var buff bytes.Buffer;
	clist[len(clist)-1].Stdout = &buff;

    for _, c := range clist { c.Start(); }

    for i := 1; i < len(clist); i++ {
		clist[i-1].Wait();
		w[i-1].Close();
		clist[i].Wait();
	}

	return strings.TrimSuffix(buff.String(), "\n");
}
