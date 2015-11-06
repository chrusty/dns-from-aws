package types

import (
	"sync"
)

type HostInventory struct {
	Environments map[string]Environment
}

type Environment struct {
	DNSRecords map[string][]string
}

// Configuration object for the HostInventoryUpdater:
type Config struct {
	HostUpdateFrequency int
	DNSUpdateFrequency  int
	RoleTag             string
	EnvironmentTag      string
	DNSDomainName       string
	AWSRegion           string
	DNSTTL              int64
	HostInventory       HostInventory
	HostInventoryMutex  sync.Mutex
}
