package testenv

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

func AfterEachSleep() {
	p := root + afterEachSleepPath
	if _, err := os.Stat(p); err == nil {
		if data, err := ioutil.ReadFile(p); err == nil {
			if s, err := strconv.Atoi(string(data)); err == nil {
				t := time.Duration(s) * time.Second
				fmt.Printf("Found '%s', sleeping for '%s'", p, t)
				time.Sleep(t)
			}
		}
	}
}
