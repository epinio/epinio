package web

import (
	"net"
	"strconv"

	"github.com/suse/carrier/internal/api"
	"github.com/webview/webview"
)

func StartGui(listeningPort int) error {
	// TODO: use 0.0.0.0 to allow access from outside?
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(listeningPort))
	if err != nil {
		return err
	}

	go api.StartServer(listener)

	debug := true
	w := webview.New(debug)
	defer w.Destroy()
	w.SetTitle("Carrier")
	w.SetSize(800, 600, webview.HintNone)
	w.Navigate("http://" + listener.Addr().String())
	w.Run()

	return nil
}
