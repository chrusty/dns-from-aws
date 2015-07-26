package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/route53"
	// "encoding/xml"
	"os"
	"time"
)

// Store multi-part file (avoids blowing the memory by loading a huge file):
func dnsUpdater() {

	log.Infof("[dnsUpdater] Starting up")

	// Run forever:
	for {

		// Sleep until the next run:
		log.Debugf("[dnsUpdater] Sleeping for %vs...", dnsUpdateFrequencySeconds)
		time.Sleep(time.Duration(dnsUpdateFrequencySeconds) * time.Second)

		// Authenticate with AWS:
		awsAuth, err := aws.GetAuth("", "", "", time.Now())
		if err != nil {
			log.Criticalf("[dnsUpdater] Unable to authenticate to AWS! (%s) ...\n", err)
			os.Exit(1)

		} else {
			log.Debugf("[dnsUpdater] Authenticated to AWS")
		}

		// Make a new EC2 connection:
		log.Debugf("[dnsUpdater] Connecting to Route53...")
		route53Connection, err := route53.NewRoute53(awsAuth)
		if err != nil {
			log.Errorf("[dnsUpdater] Unable to connect to Route53! (%s) ...\n", err)
			os.Exit(1)
		}

		// Lock the host-list (so we don't try to access it when another go-routine is modifying it):
		hostInventoryMutex.Lock()

		// Make an empty batch of changes:
		changes := make([]route53.ResourceRecordSet, 0)

		// Go through each environment:
		for environmentName, environment := range hostInventory.Environments {

			log.Debugf("[dnsUpdater] Creating requests for the '%v' environment...", environmentName)

			// Now iterate over the host-inventory:
			for dnsRecordName, dnsRecordValue := range environment.DNSRecords {

				// Concatenate the parts together to form the DNS record-name:
				recordName := fmt.Sprintf("%v.%v.%v", dnsRecordName, environmentName, route53domainName)
				log.Debugf("[dnsUpdater] '%v' => '%v'", recordName, dnsRecordValue)

				// Prepare a change-request:
				resourceRecordSet := route53.BasicResourceRecordSet{
					// resourceRecordSet := route53.Change{
					Action: "UPSERT",
					Name:   recordName,
					Type:   "A",
					TTL:    recordTTL,
					Values: dnsRecordValue,
				}

				// Add it to our list of changes:
				changes = append(changes, resourceRecordSet)
			}
		}

		// Create a request to modify records:
		changeResourceRecordSetsRequest := route53.ChangeResourceRecordSetsRequest{
			Xmlns:   "https://route53.amazonaws.com/doc/2013-04-01/",
			Changes: changes,
		}

		// Submit the request:
		changeResourceRecordSetsResponse, err := route53Connection.ChangeResourceRecordSet(&changeResourceRecordSetsRequest, route53zoneId)
		if err != nil {
			log.Errorf("[dnsUpdater] Failed to make changeResourceRecordSetsResponse call: %v", err)
		} else {
			log.Infof("[dnsUpdater] Successfully updated %d DNS record-sets. Request-ID: %v, Status: %v, Submitted: %v", len(changes), changeResourceRecordSetsResponse.Id, changeResourceRecordSetsResponse.Status, changeResourceRecordSetsResponse.SubmittedAt)
		}

		// Unlock:
		hostInventoryMutex.Unlock()
	}

}