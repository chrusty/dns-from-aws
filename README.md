# dns-from-aws
Populate a DNS zone from the list of EC2 instances in your AWS account

## Features:
* Periodically interrogates the EC2 API to retrieve the list of running instances
* Uses instance-tags to determine the "role" and "environment" of each instance
* Periodically uses this list of instances to populate a DNS zone in Route53

## DNS Records:
* One internal round-robin A-record per "role" per environment using private IP addresses:
  * "webserver.us-east-1.i.test.domain.com" => [10.0.1.1, 10.0.2.1, 10.0.3.1]
* One internal round-robin A-record per "role" per AZ per environment using private IP addresses:
  * "database.us-east-1a.i.live.domain.com" => [10.2.1.11]
* One external round-robin A-record per "role" per environment using public IP addresses:
  * "gateway.us-east-1.i.staging.domain.com" => [52.12.234.13, 52.12.234.14, 52.12.234.15]

## Flags:
Usage of ./dns-from-aws:
  -awsregion="eu-west-1": The AWS region to connect to
  -dnsttl=300: TTL for any DNS records created
  -dnsupdate=60: How many seconds to sleep between updating DNS records from the host-list
  -domainname="domain.com.": The DNS domain to use (including trailing '.')
  -environmenttag="environment": Instance tag to derive the 'environment' from
  -hostupdate=60: How many seconds to sleep between updating the list of hosts from AWS
  -roletag="role": Instance tag to derive the 'role' from

## AWS Credentials:
Credentials can either be derived from IAM & Instance-profiles, or from exported key-pairs:
```
export AWS_ACCESS_KEY='AAAAAAAAAAAAAAAAAAAAAAA'
export AWS_SECRET_KEY='wrlwrlwrlwrlwrlwrwlrwlrllwrlwrwl'
```
