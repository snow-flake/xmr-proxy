package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

func main() {
	// CLI flags:
	pool_addr := flag.String("pool_addr", "xmr-us-east1.nanopool.org:14444", "XMR Pool Address")
	wallet_addr := flag.String("wallet_addr", "", "Wallet Address")

	// Parse the CLI flags
	flag.Parse()
	log.SetFlags(0)

	// Create a signal handler to monitor for SIGHUP or SIGTERM events
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, os.Kill)

	// Boot the proxy app server
	conn, err := net.Dial("tcp", *pool_addr)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	messages_sent := make(chan string)
	defer close(messages_sent)

	messages_received := make(chan string)
	defer close(messages_received)

	// Create the "done" channel, when the goroutine exits the channel will be closed
	done := make(chan struct{})

	disconnect := make(chan bool, 2)
	defer close(disconnect)

	message_from_pool := make(chan string)
	message_to_pool := make(chan string)

	worker := func() {
		defer close(done)

		reader := bufio.NewReader(conn)

		for {
			select {
			case <-disconnect:
				fmt.Println("Disconnected!")
				return
			case message := <-message_to_pool:
				i, err := fmt.Fprintf(conn, message+"\n")
				if err != nil {
					fmt.Println("Error writing message: ", err)
				} else {
					fmt.Println("Wrote message, i=", i, ", message=", message)
				}
				continue
			default:
				peek, _ := reader.Peek(1)
				if peek != nil && len(peek) > 0 {
					message, _ := reader.ReadString('\n')
					fmt.Println("Read message: ", message)
					message_from_pool <- message
				} else {
					fmt.Println("Sleeping ...")
					time.Sleep(time.Second)
					fmt.Println("Sleep over!")
				}
			}
		}
	}
	// Kickoff the worker in the background
	go worker()

	// Create a once per second timer
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	go func() {
		message_to_pool <- `{"id":1,"method":"login","params":{"agent":"xmr-stak/2.2.0/2ae7260/master/mac/cpu/monero/20","login":"` + *wallet_addr + `","pass":"x"}}`
	}()

	for {
		select {
		case <-done:
			return
		case message := <-message_from_pool:
			log.Println("Read: ", message)
		case t := <-ticker.C:
			log.Println("Tick: ", t.String())
		case <-interrupt:
			fmt.Println("Disconnecting on interrupt ...")
			disconnect <- true

			select {
			case <-done:
			case <-time.After(time.Second * 5):
			}
			fmt.Println("Disconnected on interrupt")
			return
		}

	}
}
