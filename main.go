package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var errOutOfTries = errors.New("out of tries")

type config struct {
	ttl, count int
}

func usage() {
	fmt.Printf("Usage:\n\tpb [-t ttl] [-c count] hostname\n")
}

func main() {
	switch runtime.GOOS {
	case "darwin":
	case "linux":
		break
	default:
		fmt.Printf("Go does not support ICMP on %s.\n", runtime.GOOS)
		return
	}
	config := new(config)
	flag.IntVar(&config.ttl, "t", 0, "Set the IP Time to Live.")
	flag.IntVar(&config.count, "c", 0, "Stop after sending c packets.")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Printf(
			"Wrong amount of arguments, got %d, want 1.\n",
			flag.NArg())
		usage()
		flag.PrintDefaults()
		return
	}
	sts := stats{}
	results := make(chan *result)
	errors := make(chan error)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // Handle Ctrl+C events.
	p, err := newPinger(flag.Arg(0), config)
	if err != nil {
		log.Fatal(err)
	}
	go p.start(results, errors)
	for {
		select {
		case res := <-results:
			fmt.Println(res)
			sts = append(sts, res)
		case err := <-errors:
			if err == errOutOfTries {
				fmt.Println("\r")
				p.stop()
				fmt.Println(sts)
				return
			}
			fmt.Println(err)
		case <-c:
			fmt.Println("\r")
			p.stop()
			fmt.Println(sts)
			return
		}
	}
}
