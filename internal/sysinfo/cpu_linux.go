//go:build linux

package sysinfo

import (
	"bufio"
	"os"
	"regexp"
)

func getCPUName() string {
	column := regexp.MustCompile("\t+: ")

	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	s := bufio.NewScanner(f)

	for s.Scan() {
		sl := column.Split(s.Text(), 2)
		if sl == nil {
			continue
		}

		if sl[0] == "model name" {
			return sl[1]
		}
	}

	return ""
}
