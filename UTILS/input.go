package util

import (
	"bufio"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type Input struct {
	reader   *bufio.Reader
	register chan chan string
	cmdCh    chan string
	download atomic.Bool
}

func NewInput() *Input {
	in := &Input{
		reader:   bufio.NewReader(os.Stdin),
		register: make(chan chan string),
	}
	go in.loop()
	return in
}

func (in *Input) ReadLine() string {
	ch := make(chan string, 1)
	in.register <- ch
	return <-ch
}

func (in *Input) StartDownload(cmdCh chan string) {
	in.cmdCh = cmdCh
	in.download.Store(true)
}

func (in *Input) StopDownload() {
	in.download.Store(false)
	in.cmdCh = nil
}

func (in *Input) loop() {
	for {
		if in.download.Load() {
			if StdinReady() {
				line, err := in.reader.ReadString('\n')
				if err != nil {
					return
				}
				cmd := strings.ToLower(strings.TrimSpace(line))
				if ch := in.cmdCh; ch != nil && cmd != "" {
					select {
					case ch <- cmd:
					default:
					}
				}
			}
			time.Sleep(40 * time.Millisecond)
			continue
		}
		ch := <-in.register
		line, err := in.reader.ReadString('\n')
		if err != nil {
			return
		}
		ch <- strings.TrimSpace(line)
	}
}
