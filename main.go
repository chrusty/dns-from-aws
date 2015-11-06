package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/cihub/seelog"

	dns "github.com/chrusty/dns-from-aws/dns"
	hostinventory "github.com/chrusty/dns-from-aws/hostinventory"
	types "github.com/chrusty/dns-from-aws/types"
)

var (
	roleTag        = flag.String("roletag", "role", "Instance tag to derive the 'role' from")
	environmentTag = flag.String("environmenttag", "environment", "Instance tag to derive the 'environment' from")
	dnsTTL         = flag.Int64("dnsttl", 300, "TTL for any DNS records created")
	hostUpdateFreq = flag.Int("hostupdate", 60, "How many seconds to sleep between updating the list of hosts from AWS")
	dnsUpdateFreq  = flag.Int("dnsupdate", 60, "How many seconds to sleep between updating DNS records from the host-list")
	dnsDomainName  = flag.String("domainname", "domain.com.", "The DNS domain to use (including trailing '.')")
	awsRegion      = flag.String("awsregion", "eu-west-1", "The AWS region to connect to")
)

func init() {
	// Parse the command-line arguments:
	flag.Parse()

}

func main() {
	// Make sure we flush the log before quitting:
	defer log.Flush()

	var hostInventoryMutex sync.Mutex
	var hostInventory types.HostInventory

	// Configuration object for the HostInventoryUpdater:
	config := types.Config{
		HostUpdateFrequency: *hostUpdateFreq,
		DNSUpdateFrequency:  *dnsUpdateFreq,
		RoleTag:             *roleTag,
		EnvironmentTag:      *environmentTag,
		DNSDomainName:       *dnsDomainName,
		AWSRegion:           *awsRegion,
		DNSTTL:              *dnsTTL,
		HostInventory:       hostInventory,
		HostInventoryMutex:  hostInventoryMutex,
	}

	// Run the host-inventory-updater:
	go hostinventory.Updater(&config)

	// Run the dns-updater:
	go dns.Updater(&config)

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
			log.Flush()
			os.Exit(0)
		}

	}

}
