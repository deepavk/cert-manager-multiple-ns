## Overview

Certificate management is required across two namespaces, the data plane components being in one namespace
and the control plane components in the other namespace.

## Goals

* Asses installation of the cert manager in a separate namespace
* Certification rotation and expiry without downtime
* Assess any failures during certificate renewal

## Prerequisites

* Installation of cert manager component in a namespace and services that require certificates in a different namespaces

## Observations

During the installation of cert manager using helm package it was observed that cert manager installs a set of resources in
the installation namespace as well as the default namespace. The service can consume the certificate resource when installed in the same
namespace as the certificates. The key rotation and certificate expiry handling is supported by the cert manager.

## Cert-manager resources installed

### [Issuers](https://cert-manager.io/docs/concepts/issuer/)
Cert-manager supports two types of issuers
- The cluster issuer which works across namespaces and can be linked to
  the certificate resources.
- An issuer would work within the same namespace that it is created in.
- There can be selfsigned issuers, custom issuers or acme issuers like lets encrypt
  (Testing with custom issuers or lets encrypt is to be done)

### [Certificates]( https://cert-manager.io/docs/usage/certificate/)

- Certificates can be installed in the services namespace
  and have a link to the cluster issuer
- The cert manager creates a secret to store the certificate

#### Certificate expiry and renewal

- The duration and renewBefore properties allow us to set the
  expiry time and renewal time
- The certificate renewal is triggered at the difference of the duration and renewBefore times

```   
For example if the settings for the above values are:
    duration: 1h0m1s
    renewBefore: 5m1s
Then the certificate expires at 10am and the renewal process is triggered at 9:55am
    
``` 

Describing the certificate would show these logs at the time of certificate renewal:
```
  Normal  Issuing    7m5s (x9 over 6h32m)  cert-manager  The certificate has been successfully issued
  Normal  Issuing    7m5s                  cert-manager  Renewing certificate as renewal was scheduled at 2021-04-19 10:10:29 +0000 UTC
  Normal  Generated  7m5s                  cert-manager  Stored new private key in temporary Secret resource "selfsigned-cert-tmqhr"
  Normal  Requested  7m5s                  cert-manager  Created new CertificateRequest resource "selfsigned-cert-v66db"
```

#### Key rotation

Cert manager supports private key rotation and allows the setting of the rotationPolicy to Always
or Never
```
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
name: my-cert
...
spec:
secretName: my-cert-tls
dnsNames:
- example.com
  privateKey:
  rotationPolicy: Always
```

### Setup created to test the above usecase

1. The cert manager was installed in the cert-manager namespace
2. The cluster issuer installed in the cert-manager namespace
3. Services, and their certificates are installed within their own namespace.

| Component | Namespace | 
  | --------------- | --------- |
| cert-manager  | cert-manager | 
| cluster-issuer | issuerns |
| service 1 | application |
| service 2 | application2 |

These resources will be created on application of the above yaml files in this repo

### Observation on certificate expiry

* On certificate expiry the certificates were renewed by cert-manager
* The secrets were updated with the new certificates
* The go code that starts a server and loads the certificates still points to the old certificate. There are no errors
  seen until the pods are restarted and then new certificates are loaded.

To ensure that correct certificates are referred to:

* Redeployment of components may be required
* We could have a watcher to check when secrets are updated and then refresh the certificate that the server is using.
  (go code added in this repo)


A config is passed to the server on startup. Server configurations must set one of Certificates, GetCertificate or
GetConfigForClient. Clients doing client-authentication may set either Certificates or GetClientCertificate.

For the server certificates a function can be used to retrieve certificates, the function reads the certificate
from a mutex locked pointer to the certificate.
The certificate (of type tls.Certificate) is stored in a pointer in the CertificateKeyPair structure. This pointer
can be updated when the value changes.

```
  type CertificateKeyPair struct {
      certMutex sync.RWMutex
      cert      *tls.Certificate
      certPath  string
      keyPath   string
  }
```
The certificate is dynamically loaded to the server's tls config

```
	config := &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}
	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: config,
	}
	server.TLSConfig.GetCertificate = kpr.GetCertificateFunc()
```


### Installation and deployment
The example application contains a service that requires certificates.
The certificate management is done by cert-manager.

* The service and certificates are installed in a namespace "application".
* Cert manager is installed in its default "cert-manager" namespace
* A cluster issuer resource is installed in "issuerns" namespace

A namespace "cert-manager" is created and helm chart is installed here
https://cert-manager.io/docs/installation/kubernetes/#installing-with-helm

#### Installation of ingress controller to allow accessing the service through an ingress resource
kubectl -n ingress-nginx apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v0.41.2/deploy/static/provider/cloud/deploy.yaml

#### Deploying the service
The Makefile's "deploy-example" command can be used to deploy the components 

