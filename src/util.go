package main

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
)

// Run function
func supportRun(command string) string {
	s := strings.Split(command, " ")
	out, e := exec.Command(s[0], s[1:]...).CombinedOutput()
	lg.err("command ["+command+"] error: "+string(out), e)
	return strings.TrimSuffix(string(out), "\n")
}

// RunPipe function
// This pipe program is not working on wg show | grep blah blah!
func supportRunPipe(input ...string) string {
	clist := []*exec.Cmd{}
	for _, in := range input {
		s := strings.Split(in, " ")
		clist = append(clist, exec.Command(s[0], s[1:]...))
	}

	r := make([]*io.PipeReader, len(clist)-1)
	w := make([]*io.PipeWriter, len(clist)-1)

	for i := 0; i < len(clist)-1; i++ {
		r[i], w[i] = io.Pipe()
	}

	for i := 1; i < len(clist); i++ {
		clist[i-1].Stdout = w[i-1]
		clist[i].Stdin = r[i-1]
	}

	var buff bytes.Buffer
	clist[len(clist)-1].Stdout = &buff

	for _, c := range clist {
		c.Start()
	}

	for i := 1; i < len(clist); i++ {
		clist[i-1].Wait()
		w[i-1].Close()
		clist[i].Wait()
	}

	return strings.TrimSuffix(buff.String(), "\n")
}
