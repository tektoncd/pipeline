# go-expect

[![Build Status](https://travis-ci.org/Netflix/go-expect.svg?branch=master)](https://travis-ci.org/Netflix/go-expect)
[![GoDoc](https://godoc.org/github.com/Netflix/go-expect?status.svg)](https://godoc.org/github.com/Netflix/go-expect)
[![NetflixOSS Lifecycle](https://img.shields.io/osslifecycle/Netflix/go-expect.svg)]()

Package expect provides an expect-like interface to automate control of applications. It is unlike expect in that it does not spawn or manage process lifecycle. This package only focuses on expecting output and sending input through it's pseudoterminal.

## Usage

```go
package main

import (
	"log"
	"os"
	"os/exec"
	"time"

	expect "github.com/Netflix/go-expect"
)

func main() {
	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cmd := exec.Command("vi")
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	go func() {
		c.ExpectEOF()
	}()

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(time.Second)
	c.Send("iHello world\x1b")
	time.Sleep(time.Second)
	c.Send("dd")
	time.Sleep(time.Second)
	c.SendLine(":q!")

	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
```
