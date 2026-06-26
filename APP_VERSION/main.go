package main

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"

	"shiryu/GUI_VERSION/server"
	"shiryu/pkg/ui"

	webview "github.com/webview/webview_go"
)

func main() {
	staticFS, err := fs.Sub(ui.FS, "static")
	if err != nil {
		log.Fatal(err)
	}
	srv := server.New(staticFS)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	go func() {
		if err := http.Serve(ln, srv.Handler()); err != nil {
			log.Fatal(err)
		}
	}()

	debug := false
	w := webview.New(debug)
	defer w.Destroy()
	w.SetTitle("Shiryu")
	w.SetSize(1100, 780, webview.HintNone)
	w.Navigate(url)
	w.Run()
}
