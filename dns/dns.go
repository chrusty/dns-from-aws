package dns

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/cihub/seelog"

	types "github.com/chrusty/dns-from-aws/types"

	aws "github.com/goamz/goamz/aws"
	route53 "github.com/goamz/goamz/route53"
)

// Periodically populate DNS using the host-inventory:
func Updater(config *types.Config) {

	// Get the Route53 "Zone-ID":
	route53zoneId, err := getRoute53ZoneId(config.DNSDomainName)
	if err != nil {
		log.Criticalf("Error looking up DNS zone-id: %v", err)
		os.Exit(2)
	}

	// Run forever:
	log.Infof("[dnsUpdater] Started")
	for {

		// Sleep until the next run:
		log.Debugf("[dnsUpdater] Sleeping for %vs ...", config.DNSUpdateFrequency)
		time.Sleep(time.Duration(config.DNSUpdateFrequency) * time.Second)

		// Lock the host-list (so we don't try to access it when another go-routine is modifying it):
		log.Tracef("[dnsUpdater] Trying to lock config.HostInventoryMutex ...")
		config.HostInventoryMutex.Lock()
		log.Tracef("[dnsUpdater] Locked config.HostInventoryMutex")

		// See if we actually have any changes to make:
		if len(config.HostInventory.Environments) > 0 {

			// Authenticate with AWS:
			awsAuth, err := aws.GetAuth("", "", "", time.Now())
			if err != nil {
				log.Errorf("[dnsUpdater] Unable to authenticate to AWS! (%s)", err)
				continue

			} else {
				log.Debugf("[dnsUpdater] Authenticated to AWS")
			}

			// Make a new EC2 connection:
			log.Debugf("[dnsUpdater] Connecting to Route53 ...")
			route53Connection, err := route53.NewRoute53(awsAuth)
			if err != nil {
				log.Errorf("[dnsUpdater] Unable to connect to Route53! (%s)", err)
				continue
			}

			// Go through each environment:
			for environmentName, environment := range config.HostInventory.Environments {

				// Make an empty batch of changes:
				changes := make([]route53.ResourceRecordSet, 0)

				// Now iterate over the host-inventory:
				log.Debugf("[dnsUpdater] Creating requests for the '%v' environment ...", environmentName)
				for dnsRecordName, dnsRecordValue := range environment.DNSRecords {

					// Turn the list of strings (host-addresses) into a list of route53.ResourceRecordValue:
					resourceRecordValues := make([]route53.ResourceRecordValue, 0)
					for _, hostAddress := range dnsRecordValue {
						resourceRecordValues = append(resourceRecordValues, route53.ResourceRecordValue{Value: hostAddress})
					}

					// Prepare a change-request:
					log.Debugf("[dnsUpdater] Record: %v => %v", dnsRecordName, dnsRecordValue)
					changes = append(changes, &route53.BasicResourceRecordSet{
						Action: "UPSERT",
						Name:   dnsRecordName,
						Type:   "A",
						TTL:    config.DNSTTL,
						Values: resourceRecordValues,
					})

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
					continue
				} else {
					log.Infof("[dnsUpdater] Successfully updated %d DNS record-sets for %v.%v (Request-ID: %v, Status: %v, Submitted: %v)", len(changes), environmentName, config.DNSDomainName, changeResourceRecordSetsResponse.Id, changeResourceRecordSetsResponse.Status, changeResourceRecordSetsResponse.SubmittedAt)
				}

			}

		} else {
			log.Info("[dnsUpdater] No DNS changes to make")
		}

		// Unlock the host-inventory:
		log.Tracef("[dnsUpdater] Unlocking config.HostInventoryMutex ...")
		config.HostInventoryMutex.Unlock()

	}

}

// Lookup the Route53 zone-id for the domain-name we were given:
func getRoute53ZoneId(domainName string) (string, error) {

	// Authenticate with AWS:
	awsAuth, err := aws.GetAuth("", "", "", time.Now())
	if err != nil {
		log.Criticalf("[dnsUpdater] Unable to authenticate to AWS! (%s)", err)
		return "", err

	} else {
		log.Debugf("[dnsUpdater] Authenticated to AWS")
	}

	// Make a new EC2 connection:
	log.Debugf("[dnsUpdater] Connecting to Route53 ...")
	route53Connection, err := route53.NewRoute53(awsAuth)
	if err != nil {
		log.Criticalf("[dnsUpdater] Unable to connect to Route53! (%s)", err)
		return "", err
	}

	// Submit the request:
	ListHostedZonesResponse, err := route53Connection.ListHostedZones("", 100)
	if err != nil {
		log.Criticalf("[dnsUpdater] Failed to make ListHostedZones call: %v", err)
		return "", err
	} else {
		log.Debugf("[dnsUpdater] Retreived %d DNS zones.", len(ListHostedZonesResponse.HostedZones))
	}

	// Go through the responses looking for our zone:
	for _, hostedZone := range ListHostedZonesResponse.HostedZones {
		// Compare the name to the one provided:
		if hostedZone.Name == domainName {
			log.Infof("[dnsUpdater] Found ID (%v) for domain (%v).", hostedZone.Id, domainName)

			// Split the zone-ID (because they tend to look like "/hostedzone/ZXJHAS123"):
			return strings.Split(hostedZone.Id, "/")[2], nil
			break
		}
	}

	log.Criticalf("[dnsUpdater] Couldn't find zone-ID for domain (%v)!", domainName)
	os.Exit(1)
	return "", errors.New(fmt.Sprintf("Couldn't find DNS-domain '%v' on your AWS account", domainName))

}
