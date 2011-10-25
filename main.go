package main

import (
	"crypto/tls"
	"io"
	"os"
	"bufio"
	"imap"
	"log"
	"flag"
	"fmt"
)

var verbose *bool = flag.Bool("v", false, "verbose output")
var dumpProtocol *bool = flag.Bool("dumpprotocol", false, "dump imap stream")

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

func loadAuth(path string) (string, string) {
	f, err := os.Open(path)
	check(err)
	r := bufio.NewReader(f)

	user, isPrefix, err := r.ReadLine()
	check(err)
	if isPrefix {
		panic("prefix")
	}

	pass, isPrefix, err := r.ReadLine()
	check(err)
	if isPrefix {
		panic("prefix")
	}

	return string(user), string(pass)
}

func readExtra(im *imap.IMAP) {
	for {
		select {
		case msg := <-im.Unsolicited:
			log.Printf("*** unsolicited: %T %+v", msg, msg)
		default:
			return
		}
	}
}

func connect() *imap.IMAP {
	user, pass := loadAuth("auth")

	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	check(err)

	var r io.Reader = conn
	if *dumpProtocol {
		r = newLoggingReader(r, 300)
	}
	im := imap.New(r, conn)
	im.Unsolicited = make(chan interface{}, 100)

	if *verbose {
		log.Printf("connecting")
	}
	hello, err := im.Start()
	check(err)
	if *verbose {
		log.Printf("server hello: %s", hello)
	}

	if *verbose {
		log.Printf("logging in")
	}
	resp, caps, err := im.Auth(user, pass)
	check(err)
	if *verbose {
		log.Printf("capabilities: %s", caps)
		log.Printf("%s", resp)
	}

	return im
}

func usage() {
	fmt.Printf("usage: %s command\n", os.Args[0])
	fmt.Printf("commands are:\n")
	fmt.Printf("  list   list mailboxes\n")
	fmt.Printf("  fetch  download mailbox\n")
	os.Exit(0)
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}
	mode := args[0]
	args = args[1:]

	switch mode {
	case "list":
		im := connect()
		mailboxes, err := im.List("", imap.WildcardAny)
		check(err)
		fmt.Printf("Available mailboxes:\n")
		for _, mailbox := range mailboxes {
			fmt.Printf("  %s\n", mailbox.Name)
		}
		readExtra(im)
	case "fetch":
		if len(args) < 1 {
			fmt.Printf("must specify mailbox to fetch\n")
			os.Exit(1)
		}
		mailbox := args[0]

		im := connect()
		{
			resp, err := im.Examine(mailbox)
			check(err)
			log.Printf("%s", resp)
			log.Printf("%+v", resp)
			readExtra(im)
		}

		f, err := os.Create(mailbox + ".mbox")
		check(err)
		mbox := newMbox(f)

		{
			fetches, err := im.Fetch("1:4", []string{"RFC822"})
			check(err)
			for _, fetch := range fetches {
				mbox.writeMessage(fetch.Rfc822)
			}
			readExtra(im)
		}
	default:
		usage()
	}
}
