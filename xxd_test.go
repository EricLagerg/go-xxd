package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"
)

func TestOutFile(t *testing.T) {
	f, err := ioutil.ReadFile("outfile")
	if err != nil {
		t.Error(err)
	}

	cmd := exec.Command("./xxd", "image.jpg")
	b, err := cmd.Output()
	if err != nil {
		t.Error(err)
	}

	if string(b) != string(f) {
		t.FailNow()
	}

	for i, v := range f {
		if b[i] != v {
			fmt.Printf("Expected:\n%c\n\nGot:\n%c\n%d", v, b[i], i)
			t.FailNow()
		}
	}

}
