#  AWS EKS configuration

## Create an EKS cluster

If you don't have an existing cluster, follow the [quickstart](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html) to create an EKS cluster.

## EKS Prerequisites

* We follow the documented recommendation of having at least 2 nodes for EKS
* Epinio v0.0.21 has been tested with AWS EKS incl. kubernetes version v1.20.4
* To do more extensive testing we recommend a 2 node cluster with "t3.xlarge" instances
* To just try out Epinio, e.g. 2 "t3a.large" are sufficient

#### Install Dependencies

Follow these [steps](./install_dependencies.md) to install dependencies.

#### Install Epinio CLI

* Download the binary

Find the latest version at [Releases](https://github.com/epinio/epinio/releases) and run e.g.

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/v0.0.20/epinio-linux-amd64
```

* Make the binary executable

```bash
chmod +x epinio
```

* Move the binary to your PATH

```bash
sudo mv ./epinio /usr/local/bin/epinio
```

#### Install

Installing Epinio in an EKS cluster is done in two steps.

#### Install Ingress In Cluster (for a custom DOMAIN)

Install ingress first and wait for the `loadbalancer-ip` to be provisioned for the `traefik` ingress. Then, you can map the `loadbalancer-ip` to your `Domain Name` e.g. `example.net` and wait for it to be mapped.

```bash
epinio install-ingress
```

The output of the command will print the `loadbalancer-ip`, but with EKS it will print a loadbalanced FQDN, possibly resolving to multiple IPs. Therefore we recommend to create a wildcard domain using CNAME records.

#### Example wildcard DOMAIN with AWS "route53" service

As an example we will use the [AWS Service Route53](https://console.aws.amazon.com/route53/v2/home#Dashboard) to create a wildcard domain within one of your existing "Hosted zones", e.g. **example.net**.

Given Epinio ingress installation provided you with the following hostname:

```bash
Traefik Ingress info: [{"hostname":"abcdefg12345671234567abcdefg1234-1234567890.eu-west-1.elb.amazonaws.com"}]
```

you will have to add two CNAME records, for the subdomain, e.g. "test" to have "test.example.net", resp. "\*.test.example.net".

##### test.example.net

```bash
Record name: test
Record type: CNAME - Routes traffic to another domain name and some AWS resources
Value: abcdefg12345671234567abcdefg1234-1234567890.eu-west-1.elb.amazonaws.com
```

##### \*.test.example.net

```bash
Record name: *.test
Record type: CNAME - Routes traffic to another domain name and some AWS resources
Value: abcdefg12345671234567abcdefg1234-1234567890.eu-west-1.elb.amazonaws.com
```

Finally,

`> host test.example.net`, or even

`> host epinio.test.example.net`

should resolve to e.g. "abcdefg12345671234567abcdefg1234-1234567890.eu-west-1.elb.amazonaws.com".

#### Install Epinio In Cluster

```bash
epinio install --system-domain test.example.net --tls-issuer=letsencrypt-production --use-internal-registry-node-port=false
```
