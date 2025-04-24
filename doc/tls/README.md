# TLS

Skupper uses TLS certificates to authenticate and secure communications between
skupper routers in a network through channels called Links. The Skupper
controller for Kubernetes will by default take care of issuing the TLS
credentials used by Links. This document elaborates on the requirements for
these TLS credentials, the default scheme used by the Skupper Kubernetes
controller, and how other certificate infrastructure could be included.

## TLS Credentials Requirements

Skupper uses mutual TLS (mTLS) to establish Links between Sites. Each side of
these connections, the Link and the RouterAccess that it connects to, must
present a valid certificate and validate the peer's certificate using a trusted
CA. Any intermediate load balancers between Sites should use TLS passthrough
(i.e. should not terminate TLS.) Terminating TLS will prevent the routers on
either site from authenticating one another.

### Link TLS Credentials

A Link has its own set of TLS Credentials that includes a certificate and a
database of trusted Certificate Authorities (CAs) that it will use to
authenticate the server's Certificate. The Link Certificate has few specific
requirements. It must be valid and signed by a CA the peer RouterAccess trusts,
and should have appropriate key usage attributes for client authentication.

### RouterAccess TLS Credentials

Every Skupper Link is made to the endpoints from a Site's RouterAccess, the
server side of the connection. Each RouterAccess has its own set of TLS
Credentials that includes the serving certificate for authenticating itself
with peer routers, and a database of trusted Certificate Authorities (CAs) it
uses to authenticate client Certificates.

The RouterAccess Certificate has the typical requirements for a TLS web server:
usage for digital signature key encipherment and server auth. The Certificate
also must be valid for the host(s) in the Link/RouterAccess endpoints (depends
on the ingress chosen) in order for peers to validate the connection.

> ⚠️ Known Issue: The Skupper router ignores Subject Alternative Name IP entries
> when doing hostname validation. The skupper controller works around this by
> adding IPs as DNS entries. Not all PKI tools support this.
> for example:
>
> X509v3 Subject Alternative Name:
>
>   DNS:172.18.255.193, IP Address:172.18.255.193

### Certificate Secret Layout

Skupper expects TLS Credentials in standard Kubernetes kubernetes.io/tls Secret
format with the following fields.

- **tls.crt** PEM encoded X.509 certificate - the public certificate
- **tls.key** PKCS#1 private key - the private key for the certificate
- **ca.crt** PEM encoded X.509 certificate(s), trusted Certificate Authorities
  "database". Multiple certificates can be concatenated in this field.

## Controller-Managed TLS in Kubernetes

The Skupper controller for Kubernetes will automate the creation of the TLS
Credentials for a Site's Links and RouterAccess. In this model, there is no
central authority of trust for a network. Instead, each Site acts as its own
root of trust, issuing and validating credentials independently using a
site-scoped CA.

### Issuing Site Link Access Credentials

When a Site with link access enabled is initialized:
- Skupper issues a self-signed CA named `skupper-site-ca`, valid for 5 years.
- Skupper issues a `skupper-site-server` certificate, valid for 5 years, signed
  by `skupper-site-ca`.
- The `skupper-site-ca` certificate is embedded in the `ca.crt` field of
  `skupper-site-server`.
- The RouterAccess is configured with the `skupper-site-server` TLS
  credentials. Only clients with certificates signed by `skupper-site-ca` will
  be authorized to connect.

### Issuing Link Credentials

A link can be created in several ways, either manually with `skupper link
generate` or when redeeming an AccessToken. Regardless, the issuance is done by
the RouterAccess side Site.

- The Skupper controller issues a new client TLS certificate signed by the
  RouterAccess side `skupper-site-ca` CA, and embedds the CA public key into
  the `ca.crt` field of the client certificate.
- The new client certificate is transported to the Link-side Site, either
  manually by the user or over https though the AccessToken redemption
  endpoint. It is saved as a Secret, often with a random name prefixed by the
  RouterAccess Site name.
- This Secret is then referenced in the Link’s spec.tlsCredentials field.
- The Link side routers are configured to initiate secure connections to the
  peer Site using these new credentials.

## Manual TLS Certificate Management

Understanding the requirements for TLS Credentials for connecting Sites and the
default behaviour of the Skupper controller for Kubernetes, it is possible to
exercise finer control over the certificates issued.

### Using user-provided CAs

As a rule, if a secret that the Skupper controller requires already exists, or
that secret is overwritten by a user, Skupper will stop trying to manage that
secret. This means that it is possible to provide any of the credentials
manually. One pattern that can be useful is to provide Sites with an alternate
CA in `skupper-site-ca`.

### Manually Managing Link and RouterAccess tlsCredentials

Full manual control of TLS Certificates can be accomplished by manually
managing RouterAccess and Links.

This example sets up the following:
* A toy PKI using the `openssl` tool (written using `OpenSSL 3.2.4`)
* A single network-scoped CA for simplicity - contrasts with the default site-scoped CA.
* Two Kubernetes sites in different clusters. One "public" Site that will use a
  LoadBalancer Service for ingress, and one "private" Site without ingress.

#### Issue a network CA

Use the openssl configuration file in this directory to set up a self-signed CA.
```
openssl req -x509 -new \
        -config network-ca.conf -newkey rsa \
        -out ca.crt -keyout ca.key
```

This should produce two files `ca.crt` and `ca.key`.

#### Set up the Skupper Sites

Using the kube context for the public site: `kubectl apply -f public.yaml`

This manifest has a Site and RouterAccess configured for the public site. The
RouterAccess points to `tlsCredentials: public-server-tls`. This will be the
name of the Secret Skupper will use.

Using the kube context for the private site: `kubectl apply -f private.yaml`

#### Manually Issuing Certificates

In order to issue a TLS Certificate for the public site, we first need the
address(es) that the RouterAccess will listen on. This is required for peers to
validate the host.

Get the RouterAccess Status using the kube context for the public site:
```
kubectl get routeraccesses.skupper.io skupper \
    -ojsonpath='{range .status.endpoints[*]}{.host}{"\n"}{end}'
```

Assuming that a LoadBalancer service was created, there should be a document
with endpoints. This will be important later, but for now we only need the
hosts. In this example there was one unique address `172.18.255.193`.

Now we can issue a key pair for the public site and put it in the `public-server-tls` Secret.

> ⚠️ Do not forget: substitute in your RouterAccess host in for COMMON_NAME

```
COMMON_NAME=172.18.255.193
# Generate a private key for site-public
openssl genrsa -out site-public.key 4096
# Create a signing request, setting the CN to the address
# from the RouterAccess endpoint
openssl req -new \
    -config network-ca.conf -section site \
    -subj "/CN=${COMMON_NAME:-skupper-router}/O=network:site" \
    -key site-public.key -out site-public.csr
# Issue the public site's certificate
openssl x509 -req -days 100 \
    -copy_extensions copyall \
    -CA ca.crt -CAkey ca.key \
    -in site-public.csr -out site-public.crt
```

This should produce the following files:

`site-public.key`: The private key for the public site
`site-public.crt`: The public key of the public site

Add these new credentials to the public site.

```
kubectl create secret generic public-server-tls \
    --from-file=ca.crt=ca.crt \
    --from-file=tls.crt=site-public.crt \
    --from-file=tls.key=site-public.key
```
Now we can issue a key pair for the private site to use for client authentication with the public site.

```
# Generate a private key
openssl genrsa -out site-private.key 4096
# Create a signing request for the private key
openssl req -new \
    -config network-ca.conf -section site \
    -key site-private.key -out site-private.csr
# Issue the private site's certificate
openssl x509 -req -days 100 \
    -copy_extensions copyall \
    -CA ca.crt -CAkey ca.key \
    -in site-private.csr -out site-private.crt
```

This should produce the following files:

`site-private.key`: The private key for the private site
`site-private.crt`: The public key of the private site

Add these new credentials to the private site.
```
kubectl create secret generic public-link-tls \
    --from-file=ca.crt=ca.crt \
    --from-file=tls.crt=site-private.crt \
    --from-file=tls.key=site-private.key
```

#### Link Sites

At this point the public site should have a working RouterAccess and the
private site should have TLS credentials that will allow it to authenticate.
The private site needs a Link resource to complete the connection.

The Link document needs to be populated with `tlsCredentials: public-link-tls`
to match the name of the secret in the private site to use. It also requires a
`endpoints` block, identical to the one in the public site's RouterAccess
status section. The skupper CLI can generate this document for you, using the
public kube context.

```
skupper link generate \
    --tls-credentials public-link-tls \
    --generate-credential=false | tee link.yaml
```

This will output a Link record document with the endpoints filled to connect to
the public site.

Using the kube context for the private site: `kubectl apply -f link.yaml`

This will apply the link to the private site. The sites should connect.

## TLS Credential Rotation

**TODO** Support for rotating TLS Credentials is currently under development
and is not yet fully implemented.

When considering rotating the TLS credentials used by Skupper, it is important
to understand the trust model used for linking Skupper Sites. The default
[trust model](#controller-managed-tls-in-kubernetes) used by Skupper is
distributed: each Site having its own trust root. Because of this, rotating
client certificates and Site CAs is an especially complicated exercise.

## Troubleshooting TLS Issues

TLS errors logged by Skupper routers are relatively common, and do not always
indicate a problem on their own. Because Skupper routers use TLS connections
for everything, ANY connectivity issue _becomes_ a TLS error. For example, any
router in the network getting rescheduled or stopped will likely manifest a TLS
error somewhere.

When there is a Link in the network that appears broken, begin by verifying
connectivity before looking at TLS specific issues.

- Check that the destination Site Status for problems.
  - Is the Site Ready? Are the skupper-router deployment(s) running?
  - Does the Site Status Endpoints match what is configured in the Link?
  - Is the ingress for the router ready? This depends on your controller and
    site configuration. Could be a LoadBalancer or NodePort Service, OpenShift
    Route, Gateway API TLSRoute, etc.
- Check for connectivity problems to the Link endpoints.
  - Find the host:port combinations from the Link's endpoints
  - Use a TCP client to test connectivity, ideally from the same namespace as
    the Link's Site to catch any network policy issues.
    - `echo "hello" | nc <host> <port>` Sends nonsense to the router. Expect a
      router to respond with an AMQPS error. Same for `telnet`.
    - `curl --insecure https://<host>:<port>` attempts to open a TLS connection
      (for an http request). Expect a router to refuse the connection due to
      missing client certificates. curl should print an SSL error.
    - `openssl s_client -showcerts -connect <host>:<port>` Displays diagnostics
      about an attempt to open a TLS connection. Expect a router to refuse the
      connection due to missing client certificates.

When connectivity checks out, it can be helpful to look into validating TLS
Certificates on either side of the connection.

A quick test can be done using the Link side router deployment. The following
command uses the openssl tool to attempt to open a TLS connection to a peer
router, and displays detailed diagnostics. Substitute the name of the TLS
Credentials secret used by the link, the Link endpoint host, and the relevant
port (likely 55671.)
```
TLS_SECRET_NAME=public-link-tls
LINK_ENDPOINT_HOST=172.18.255.193
PORT=55671
PROFILE_DIR="/etc/skupper-router-certs/${TLS_SECRET_NAME}-profile"
kubectl exec -it deployments/skupper-router \
    -c router -- \
        openssl s_client \
        -CAfile "$PROFILE_DIR/ca.crt" \
        -cert "$PROFILE_DIR/tls.crt" \
        -key "$PROFILE_DIR/tls.key" \
        -connect "$LINK_ENDPOINT_HOST:$PORT"  < /dev/null
```

