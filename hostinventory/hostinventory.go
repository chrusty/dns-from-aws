package hostinventory

import (
	"fmt"
	"time"

	log "github.com/cihub/seelog"

	types "github.com/chrusty/dns-from-aws/types"

	aws "github.com/goamz/goamz/aws"
	ec2 "github.com/goamz/goamz/ec2"
)

// Periodically populate the host-inventory:
func Updater(config *types.Config) {

	log.Infof("[hostInventoryUpdater] Started")

	updateFrequency := 5

	// Run forever:
	for {

		// Sleep until the next run:
		log.Debugf("[hostInventoryUpdater] Sleeping for %vs ...", updateFrequency)
		time.Sleep(time.Duration(updateFrequency) * time.Second)

		// Authenticate with AWS:
		awsAuth, err := aws.GetAuth("", "", "", time.Now())
		if err != nil {
			log.Errorf("[hostInventoryUpdater] Unable to authenticate to AWS! (%s)", err)
			continue
		} else {
			log.Debugf("[hostInventoryUpdater] Authenticated to AWS")
		}

		// Make a new EC2 connection:
		log.Debugf("[hostInventoryUpdater] Connecting to EC2 ...")
		ec2Connection := ec2.New(awsAuth, aws.Regions[config.AWSRegion])

		// Prepare a filter:
		filter := ec2.NewFilter()
		filter.Add("instance-state-name", "running")

		// Make a "DescribeInstances" call (lists ALL instances in your account):
		describeInstancesResponse, err := ec2Connection.DescribeInstances([]string{}, filter)
		if err != nil {
			log.Errorf("[hostInventoryUpdater] Failed to make describe-instances call: %v", err)
		} else {
			log.Debugf("[hostInventoryUpdater] Found %v instances running in your account", len(describeInstancesResponse.Reservations))

			// Lock the host-list (so we don't change it while another goroutine is using it):
			log.Tracef("[hostInventoryUpdater] Trying to lock config.HostInventoryMutex ...")
			config.HostInventoryMutex.Lock()
			log.Tracef("[hostInventoryUpdater] Locked config.HostInventoryMutex")

			// Clear out the existing host-inventory:
			config.HostInventory = types.HostInventory{
				Environments: make(map[string]types.Environment),
			}

			// Re-populate it from the describe instances response:
			for _, reservation := range describeInstancesResponse.Reservations {

				// Search for our role and environment tags:
				var role, environment string
				for _, tag := range reservation.Instances[0].Tags {
					if tag.Key == config.RoleTag {
						role = tag.Value
					}
					if tag.Key == config.EnvironmentTag {
						environment = tag.Value
					}
				}

				// Make sure we have environment and role tags:
				if environment == "" || role == "" {
					log.Debugf("[hostInventoryUpdater] Instance (%v) must have both 'environment' and 'role' metadata in order for DNS records to be creted!", reservation.Instances[0].InstanceId)

					// Continue with the next instance:
					continue
				} else {
					log.Infof("[hostInventoryUpdater] Building records for instance (%v) in zone (%v) ...", reservation.Instances[0].InstanceId, reservation.Instances[0].AvailabilityZone)
				}

				// Add a new environment to the inventory (unless we already have it):
				if _, ok := config.HostInventory.Environments[environment]; !ok {
					config.HostInventory.Environments[environment] = types.Environment{
						DNSRecords: make(map[string][]string),
					}
				}

				// Either create or add to the role-per-zone record:
				internalZoneRecord := fmt.Sprintf("%v.%v.i.%v.%v", role, reservation.Instances[0].AvailabilityZone, environment, config.DNSDomainName)
				if _, ok := config.HostInventory.Environments[environment].DNSRecords[internalZoneRecord]; !ok {
					config.HostInventory.Environments[environment].DNSRecords[internalZoneRecord] = []string{reservation.Instances[0].PrivateIPAddress}
				} else {
					config.HostInventory.Environments[environment].DNSRecords[internalZoneRecord] = append(config.HostInventory.Environments[environment].DNSRecords[internalZoneRecord], reservation.Instances[0].PrivateIPAddress)
				}

				// Either create or add to the role-per-region record:
				internalRegionRecord := fmt.Sprintf("%v.%v.i.%v.%v", role, config.AWSRegion, environment, config.DNSDomainName)
				if _, ok := config.HostInventory.Environments[environment].DNSRecords[internalRegionRecord]; !ok {
					config.HostInventory.Environments[environment].DNSRecords[internalRegionRecord] = []string{reservation.Instances[0].PrivateIPAddress}
				} else {
					config.HostInventory.Environments[environment].DNSRecords[internalRegionRecord] = append(config.HostInventory.Environments[environment].DNSRecords[internalRegionRecord], reservation.Instances[0].PrivateIPAddress)
				}

				// Either create or add to the external record:
				if reservation.Instances[0].IPAddress != "" {
					externalRecord := fmt.Sprintf("%v.%v.e.%v.%v", role, config.AWSRegion, environment, config.DNSDomainName)
					if _, ok := config.HostInventory.Environments[environment].DNSRecords[externalRecord]; !ok {
						config.HostInventory.Environments[environment].DNSRecords[externalRecord] = []string{reservation.Instances[0].IPAddress}
					} else {
						config.HostInventory.Environments[environment].DNSRecords[externalRecord] = append(config.HostInventory.Environments[environment].DNSRecords[externalRecord], reservation.Instances[0].IPAddress)
					}
				}

			}

		}

		// Unlock the host-inventory:
		log.Tracef("[hostInventoryUpdater] Unlocking config.HostInventoryMutex ...")
		config.HostInventoryMutex.Unlock()

		// Now set the sleep time to the correct value:
		updateFrequency = config.HostUpdateFrequency

	}

}
