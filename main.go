package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"container-agent/module"
	httpServer "container-agent/server/http"

	"github.com/go-co-op/gocron"
	"github.com/labstack/gommon/log"
	"github.com/spf13/pflag"
)

func printUsage() {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: aws-s3-client <options>")
	fmt.Fprintln(os.Stderr, "")

	pflag.Usage()
}

func main() {
	// flags
	httpPort := pflag.Uint16P("port", "P", 8080, "HTTP API Port")

	pflag.ErrHelp = errors.New("")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if *httpPort == 0 {
		fmt.Fprintln(os.Stderr, "Error: wrong HTTP API Port")
		printUsage()
		os.Exit(1)
	}
	// cron
	cronScheduler := gocron.NewScheduler(time.Local)
	delayTime := time.Now().Add(5 * time.Second)
	cronScheduler.Every(1 * time.Minute).StartAt(delayTime).SingletonMode().Do(
		func() {
			fmt.Println("Monitoring")
			module.Monitoring()
		})
	cronScheduler.StartAsync()
	defer cronScheduler.Stop()
	// http server
	hServer, err := httpServer.Start(*httpPort, log.INFO)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		return
	}
	if hServer == nil {
		fmt.Fprintln(os.Stderr, "Error: hServer == nil")
		return
	}
	defer hServer.Stop()

	// grpc client

	// signal
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)

		fmt.Println("Stoping...")

		cronScheduler.Stop()
		hServer.Stop()

		done <- true
	}()

	fmt.Println("Ready...")
	<-done
	fmt.Println("Exiting...")
}
