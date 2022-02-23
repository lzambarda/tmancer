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
	"github.com/pkg/errors"
)

const usage = `Usage is: tmancer <config>`

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
		fmt.Printf("tmancer version %s\n", internal.Version)
		os.Exit(0)
	}

	configs := []internal.TunnelConfig{}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(errors.Wrapf(err, "reading file %s", os.Args[1]))
	}
	err = json.Unmarshal(b, &configs)
	if err != nil {
		panic(errors.Wrap(err, "unmarshaling configs"))
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

	const (
		headerFormat = "%-16s%-10s%-10s%-10s%-10s%-10s\n"
		rowFormat    = "%-16s%-10s%-10d%-10s%-10s%-10s%s\n"
		notAvailable = "N/A"
	)
	fmt.Printf(headerFormat, "NAME", "TYPE", "PORT", "PID", "AGE", "STATUS")
	go func() {
		for {
			// Avoid overwriting the waiting message
			select {
			case <-ctx.Done():
				return
			default:
			}
			m.RLock()
			for i, c := range configs {
				pid := notAvailable
				if t := wrappers[i].GetPid(); t != 0 {
					pid = strconv.Itoa(t)
				}
				ageStr := notAvailable
				if age, valid := wrappers[i].GetAge(); valid {
					ageStr = age.String()
				}
				fmt.Printf(rowFormat, c.Name, c.GetType(), c.LocalPort, pid, ageStr, wrappers[i].GetStatus(), wrappers[i].GetError())
			}
			m.RUnlock()
			time.Sleep(5 * time.Second)
			fmt.Print(cursor.MoveUp(len(configs)))
		}
	}()

	<-ctx.Done()
	fmt.Println("\nWaiting for processes to end")
	wg.Wait()
	fmt.Println("Done")
}
