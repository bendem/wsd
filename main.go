package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/fatih/color"
	"golang.org/x/net/websocket"
)

// Version is the current version.
const Version = "0.1.0"

var (
	origin             string
	url                string
	protocol           string
	userAgent          string
	displayHelp        bool
	displayVersion     bool
	bufSize            int
	insecureSkipVerify bool
	raw                bool
	red                = color.New(color.FgRed).SprintFunc()
	magenta            = color.New(color.FgMagenta).SprintFunc()
	green              = color.New(color.FgGreen).SprintFunc()
	yellow             = color.New(color.FgYellow).SprintFunc()
	cyan               = color.New(color.FgCyan).SprintFunc()
	wg                 sync.WaitGroup
)

func init() {
	flag.StringVar(&origin, "origin", "http://localhost/", "origin of WebSocket client")
	flag.StringVar(&url, "url", "ws://localhost:1337/ws", "WebSocket server address to connect to")
	flag.StringVar(&protocol, "protocol", "", "WebSocket subprotocol")
	flag.StringVar(&userAgent, "userAgent", "", "User-Agent header")
	flag.BoolVar(&insecureSkipVerify, "insecureSkipVerify", false, "Skip TLS certificate verification")
	flag.BoolVar(&displayHelp, "help", false, "Display help information about wsd")
	flag.BoolVar(&displayVersion, "version", false, "Display version number")
	flag.BoolVar(&raw, "raw", false, "Don't format the messages received and don't launch an interactive shell")
	flag.IntVar(&bufSize, "bufSize", 1024, "Inbound messages buffer size")
}

func inLoop(ws *websocket.Conn, errors chan<- error, in chan<- []byte) {
	var msg = make([]byte, bufSize)

	for {
		n, err := ws.Read(msg)

		if err != nil {
			errors <- err
			continue
		}

		in <- msg[:n]
	}
}

func printErrors(errors <-chan error) {
	for err := range errors {
		if err == io.EOF {
			fmt.Fprintf(os.Stderr, "\r✝ %v - connection closed by remote\n", magenta(err))
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "\rerr %v\n", red(err))
			if !raw {
				fmt.Printf("> ")
			}
		}
	}
}

func printReceivedMessages(in <-chan []byte) {
	for msg := range in {
		if raw {
			fmt.Printf("%s", string(msg))
		} else {
			fmt.Printf("\r< %s\n> ", cyan(string(msg)))
		}
	}
}

func outLoop(ws *websocket.Conn, out <-chan []byte, errors chan<- error) {
	for msg := range out {
		_, err := ws.Write(msg)
		if err != nil {
			errors <- err
		}
	}
}

func dial(url, protocol, origin string) (ws *websocket.Conn, err error) {
	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		return nil, err
	}
	if protocol != "" {
		config.Protocol = []string{protocol}
	}
	if userAgent != "" {
		config.Header.Add("User-Agent", userAgent)
	}
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
	}
	return websocket.DialConfig(config)
}

func main() {
	flag.Parse()

	if displayVersion {
		fmt.Fprintf(os.Stdout, "%s version %s\n", os.Args[0], Version)
		os.Exit(0)
	}

	if displayHelp {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	ws, err := dial(url, protocol, origin)

	if !raw {
		if protocol != "" {
			fmt.Printf("connecting to %s via %s from %s...\n", yellow(url), yellow(protocol), yellow(origin))
		} else {
			fmt.Printf("connecting to %s from %s...\n", yellow(url), yellow(origin))
		}
	}

	defer ws.Close()

	if err != nil {
		panic(err)
	}

	if !raw {
		fmt.Printf("successfully connected to %s\n\n", green(url))
	}

	in := make(chan []byte)
	wg.Add(1)
	defer close(in)

	errors := make(chan error)
	wg.Add(1)
	defer close(errors)

	if !raw {
		out := make(chan []byte)
		wg.Add(1)
		defer close(out)

		go outLoop(ws, out, errors)

		scanner := bufio.NewScanner(os.Stdin)

		fmt.Print("> ")
		for scanner.Scan() {
			out <- []byte(scanner.Text())
			fmt.Print("> ")
		}
	}

	go inLoop(ws, errors, in)
	go printReceivedMessages(in)
	go printErrors(errors)

	wg.Wait()
}
