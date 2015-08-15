package main

type HostInventoryDNSRecords struct {
	Environments map[string]Environment
}

type Environment struct {
	DNSRecords map[string][]string
}
