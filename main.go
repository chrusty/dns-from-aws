package main

import (
	log "github.com/cihub/seelog"
	"github.com/goamz/goamz/route53"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	roleTag                             = "role"
	environmentTag                      = "environment"
	route53zoneId                       = "Z39GZVWFWTA4N8"
	route53domainName                   = "bisn.biz"
	recordTTL                           = 300
	awsRegion                           = "eu-west-1"
	hostInventoryUpdateFrequencySeconds = 15
)

var (
	hostInventoryMutex sync.Mutex
	hostInventory      HostInventoryDNSRecords
)

type HostInventoryDNSRecords struct {
	Environments map[string]Environment
}

type Environment struct {
	DNSRecords map[string][]route53.ResourceRecordValue
}

func main() {
	// Make sure we flush the log before quitting:
	defer log.Flush()

	// Update the host-inventory:
	go hostInventoryUpdater()

	// Update DNS records for the discovered hosts:
	go dnsUpdater()

	// Run until we get a kill-signal:
	runUntilKillSignal()
}

// Wait for a signal from the OS:
func runUntilKillSignal() {

	// Intercept quit signals:
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle stop / quit events:
	for {
		select {

		case <-sigChan:
			log.Infof("Bye!")
			os.Exit(0)
		}

	}

}
