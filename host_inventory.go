package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/ec2"
	"github.com/goamz/goamz/route53"
	"os"
	"time"
)

// Store multi-part file (avoids blowing the memory by loading a huge file):
func hostInventoryUpdater() {

	log.Infof("[hostInventoryUpdater] Starting up")

	// Run forever:
	for {

		// Authenticate with AWS:
		awsAuth, err := aws.GetAuth("", "", "", time.Now())
		if err != nil {
			log.Criticalf("[hostInventoryUpdater] Unable to authenticate to AWS! (%s) ...\n", err)
			os.Exit(1)

		} else {
			log.Debugf("[hostInventoryUpdater] Authenticated to AWS")
		}

		// Make a new EC2 connection:
		log.Debugf("[hostInventoryUpdater] Connecting to EC2...")
		ec2Connection := ec2.New(awsAuth, aws.Regions[awsRegion])

		// Prepare a filter:
		filter := ec2.NewFilter()
		filter.Add("instance-state-name", "running")

		// Make a "DescribeInstances" call (lists ALL instances in your account):
		describeInstancesResponse, err := ec2Connection.DescribeInstances([]string{}, filter)
		if err != nil {
			log.Criticalf("[hostInventoryUpdater] Failed to make describe-instances call: %v", err)

		} else {
			log.Debugf("[hostInventoryUpdater] Found %v instances running in your account", len(describeInstancesResponse.Reservations))

			// Lock the host-list (so we don't change it while another goroutine is using it):
			hostInventoryMutex.Lock()

			// Clear out the existing host-inventory:
			hostInventory = HostInventoryDNSRecords{
				Environments: make(map[string]Environment),
			}

			// Re-populate it from the describe instances response:
			for _, reservation := range describeInstancesResponse.Reservations {

				// Search for our role and environment tags:
				var role, environment string
				for _, tag := range reservation.Instances[0].Tags {
					if tag.Key == roleTag {
						role = tag.Value
					}
					if tag.Key == environmentTag {
						environment = tag.Value
					}
				}

				// Make sure we have environment and role tags:
				if environment == "" || role == "" {
					log.Debugf("Instance (%v) must have both 'environment' and 'role' tags in order for DNS records to be creted...", reservation.Instances[0].InstanceId)

					// Continue with the next instance:
					continue
				}

				// Either create or add to the environment record:
				if _, ok := hostInventory.Environments[environment]; !ok {
					hostInventory.Environments[environment] = Environment{
						DNSRecords: make(map[string][]route53.ResourceRecordValue),
					}
				}

				// Either create or add to the per-role record:
				roleRecord := fmt.Sprintf("%v.%v", role, awsRegion)
				if _, ok := hostInventory.Environments[environment].DNSRecords[roleRecord]; !ok {
					hostInventory.Environments[environment].DNSRecords[roleRecord] = []route53.ResourceRecordValue{{Value: reservation.Instances[0].PrivateIPAddress}}
				} else {
					hostInventory.Environments[environment].DNSRecords[roleRecord] = append(hostInventory.Environments[environment].DNSRecords[roleRecord], route53.ResourceRecordValue{Value: reservation.Instances[0].PrivateIPAddress})
				}

				// Either create or add to the role-per-az record:
				azRecord := fmt.Sprintf("%v.%v", role, reservation.Instances[0].AvailabilityZone)
				if _, ok := hostInventory.Environments[environment].DNSRecords[azRecord]; !ok {
					hostInventory.Environments[environment].DNSRecords[azRecord] = []route53.ResourceRecordValue{{Value: reservation.Instances[0].PrivateIPAddress}}
				} else {
					hostInventory.Environments[environment].DNSRecords[azRecord] = append(hostInventory.Environments[environment].DNSRecords[azRecord], route53.ResourceRecordValue{Value: reservation.Instances[0].PrivateIPAddress})
				}

			}

			// Unlock:
			hostInventoryMutex.Unlock()

		}

		// Sleep until the next run:
		log.Debugf("[hostInventoryUpdater] Sleeping for %vs...", hostInventoryUpdateFrequencySeconds)
		time.Sleep(time.Duration(hostInventoryUpdateFrequencySeconds) * time.Second)

	}

}
