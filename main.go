// Package main is the entrypoint of tmancer.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ahmetb/go-cursor"

	"github.com/lzambarda/tmancer/internal"
)

const usage = `Usage is: tmancer <config>`

// version of the program, manually set since go install.
const version = "v0.3.0"

func main() {
	if len(os.Args) != 2 {
		fmt.Println(usage)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "help", "-h", "--help":
		fmt.Println(usage)
		os.Exit(0)
	case "version", "-v", "--version":
		fmt.Printf("tmancer version %s\n", version)
		os.Exit(0)
	}

	configs := []internal.TunnelConfig{}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("reading file %s: %w", os.Args[1], err))
	}
	err = json.Unmarshal(b, &configs)
	if err != nil {
		panic(fmt.Errorf("unmarshaling configs: %w", err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	// Start all the wrappers goroutines.
	wrappers := make([]*internal.Tunnel, len(configs))
	wg := &sync.WaitGroup{}
	m := &sync.RWMutex{}
	for i, config := range configs {
		wrappers[i] = internal.NewTunnel(config)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wrappers[i].Start(ctx, m)
		}(i)
	}

	go printStatusTable(ctx, wrappers, m)

	<-ctx.Done()
	fmt.Println("\nWaiting for processes to end")
	wg.Wait()
	fmt.Println("Done")
}

const (
	headerFormat = "%-16s%-10s%-10s%-10s%-10s%-10s\n"
	rowFormat    = "%-16s%-10s%-10d%-10s%-10s%-10s%s\n"
	notAvailable = "N/A"
)

func printStatusTable(ctx context.Context, wrappers []*internal.Tunnel, m *sync.RWMutex) {
	fmt.Printf(headerFormat, "NAME", "TYPE", "PORT", "PID", "AGE", "STATUS")
	for {
		// Avoid overwriting the waiting message
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.RLock()
		for i, w := range wrappers {
			pid := notAvailable
			if t := wrappers[i].GetPid(); t != 0 {
				pid = strconv.Itoa(t)
			}
			ageStr := notAvailable
			if age, valid := wrappers[i].GetAge(); valid {
				ageStr = age.String()
			}
			name := w.GetName()
			if len(name) > 16 {
				name = name[:13] + "..."
			}
			fmt.Printf(rowFormat, name, w.GetType(), w.GetLocalPort(), pid, ageStr, wrappers[i].GetStatus(), wrappers[i].GetError())
		}
		m.RUnlock()
		time.Sleep(5 * time.Second)
		fmt.Print(cursor.MoveUp(len(wrappers)))
	}
}
