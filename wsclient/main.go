package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/errors"
	"cloud.google.com/go/trace"
	"github.com/Pallinder/go-randomdata"
	"github.com/gorilla/websocket"
	"github.com/sl1pm4t/junk/shared"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var proj = flag.String("project", "", "GCP Project ID")
var traceClient *trace.Client
var errorsClient *errors.Client

type client struct {
	c *websocket.Conn
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// create StackDriver trace client
	ctx := context.Background()
	traceClient, _ = trace.NewClient(ctx, *proj)
	errorsClient, _ = errors.NewClient(ctx, *proj, "ws", "v1.0", true)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/event"}
	log.Printf("connecting to %s", u.String())

	span := traceClient.NewSpan("wsclient-connect")
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	span.Finish()
	defer c.Close()

	done := make(chan struct{})

	// receive loop
	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second * time.Duration(5))
	defer ticker.Stop()
	bomb := time.NewTicker(time.Second * time.Duration(30))
	defer bomb.Stop()

	stdinChan := make(chan string)
	defer close(stdinChan)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			stdinChan <- scanner.Text()
		}
	}()

	for {
		ctx := context.Background()
		randProfile := randomdata.GenerateProfile(randomdata.RandomGender)
		select {
		case t := <-ticker.C:
			ev := shared.Event{
				"type": shared.EventType(),
				"ts":   t.String(),
				"user": randProfile,
			}

			send(ctx, c, ev)

		case t := <-bomb.C:
			fmt.Printf("BOOM? - %s", t)
			send(ctx, c, nil)

		case t := <-stdinChan:
			ev := shared.Event{
				"text": t,
				"type": shared.EventType(),
				"ts":   fmt.Sprintf("%s", time.Now()),
				"user": randProfile,
			}

			send(ctx, c, ev)

		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}
}

func send(ctx context.Context, c *websocket.Conn, ev interface{}) {
	defer errorsClient.Catch(ctx)

	if ev == nil {
		panic("ev is nil")
	}

	span := traceClient.NewSpan("wsclient-send")
	defer span.Finish()
	err := c.WriteJSON(ev)
	if err != nil {
		log.Println("send err:", err)
		return
	}
}
