package main

import (
	"io/ioutil"
	"os"
	"strings"
)

// File2Str function
func supportFile2Str(path string) string {
	tmp, e := ioutil.ReadFile(path)
	lg.err("No such file!: "+path+" Error msg: "+path, e)
	return strings.TrimSuffix(string(tmp), "\n")
}

// Str2File function
func supportStr2File(path, data string, permission uint32) error {
	return ioutil.WriteFile(path, []byte(data), os.FileMode(permission))
}

// Str2FileAppend function
func supportStr2FileAppend(path, data string, permission uint32) error {
	f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.FileMode(permission))
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.WriteString(data)
	if e != nil {
		return e
	}
	return nil
}
