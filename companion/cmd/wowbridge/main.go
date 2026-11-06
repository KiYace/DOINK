// Command wowbridge forwards a single DOINK JSON event or screenshot to
// the WoW AI Companion API. It is an offline integration smoke-test tool,
// not yet a background screenshot watcher.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ryan-flan/DOINK/companion/internal/bridge"
	"github.com/ryan-flan/DOINK/companion/internal/savedvars"
	"github.com/ryan-flan/DOINK/companion/internal/shot"
)

func main() {
	api := flag.String("api", "http://127.0.0.1:8080/api/events", "Symfony events endpoint")
	eventFile := flag.String("event", "", "DOINK JSON event file")
	screenshot := flag.String("screenshot", "", "DOINK-encoded screenshot")
	flag.Parse()
	if (*eventFile == "") == (*screenshot == "") {
		fmt.Fprintln(os.Stderr, "specify exactly one of --event or --screenshot")
		os.Exit(2)
	}
	var raw []byte
	if *eventFile != "" {
		var err error
		raw, err = os.ReadFile(*eventFile)
		if err != nil {
			fail(err)
		}
	} else {
		msg, ok, err := shot.DecodeFile(*screenshot)
		if err != nil {
			fail(err)
		}
		if !ok || msg.Hello {
			fail(fmt.Errorf("not a DOINK event screenshot"))
		}
		raw = msg.Payload
	}
	event, err := savedvars.DecodeEvent(raw)
	if err != nil {
		fail(err)
	}
	if err := (bridge.Client{Endpoint: *api}).Send(context.Background(), event); err != nil {
		fail(err)
	}
	fmt.Printf("sent DOINK %s #%d as Symfony player_died\n", event.Type, event.Seq)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wowbridge:", err)
	os.Exit(1)
}
