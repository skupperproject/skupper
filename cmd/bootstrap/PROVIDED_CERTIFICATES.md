# User provided certificates

Users can provide their own certificates to be used with Skupper V2 in non kube sites
during the bootstrap of a local site, when preparing a site bundle and even while installing
a site bundle at a remote machine. 

## How certificates are used internally

During the initialization of a local non kube site or when a site bundle is being prepared,
Skupper generates an internal Certificate Authority (CA) named `skupper-site-ca`.

If a `RouterAccess` is defined as part of the non kube site definition, Skupper will also
generate a server certificate that is valid for the provided `RouterAccess.spec.bindHost` and
for each entry in the  `RouterAccess.spec.subjectAlternativeName` list. The respective certificate
is signed by the CA mentioned earlier (skupper-site-ca).

A set of static links are also created for the provided `RouterAccess.spec.bindHost` and for each
entry in the  `RouterAccess.spec.subjectAlternativeName` list as separate YAML files.

These static links are composed by a `Link` and a `Secret`, which is basically the client certificate
signed by `skupper-site-ca`, that will be used by other sites to establish Skupper links.

After a site bundle has been produced, it contains the whole site definition that can be installed
at a remote location. Thus, all certificates and static links are already part of the bundle and no
new certificate is signed at the moment a bundle is installed.

## Providing your own certificates

You can provide your own certificates to be used by Skupper for site linking.
Depending on your goals, some certificates should be supplied at certain phases,
for example, at the time a local site is being initialized, a bundle is being prepared
or a bundle is being installed at a remote location.

To understand it better, let's go through the main use cases and review what is the ideal
phase that a given kind of certificate should be provided.

### Using a custom CA to sign certificates

If you want Skupper to generate client and server certificates signed by a custom CA,
you will need to provide the respective certificates during:

* Local site initialization time
* Site bundle preparation time

Certificates are signed by Skupper at the time a local namespace is being initialized
or a bundle is being produced.

After a site bundle has been produced, it already contains the whole definition and no
new certificate is expected to be signed.

### Using custom server and client certificates

If you want a new local site to use your custom server and client certificates, you can
provide them at any time, for example:

* Local site initialization time
* Site bundle preparation time
* Site bundle installation time

During "Local site initialization time" or "Site bundle preparation time", Skupper will detect
that a server certificate has been provided and will inspect it to determine its subject
alternative names and based on that, a set of static links will be created, allowing those
links to be distributed to target sites accordingly. For the static links to remain valid,
the expected client certificate must also be provided.

If a server certificate is provided at "Site bundle installation time", Skupper will also try to
determine its subject alternative names using the `openssl` binary (only if available) and it will
use it to generate the static links for the bundle installation. This way, the set of static links
available for an installed bundle will be valid for all expected target hostnames and IP addresses
defined through the server certificate.

Again, if a server certificate is provided, the respective client certificate is also expected
so that the static links have valid client credentials.

## Examples

## Provide your own skupper-site-ca

If you want Skupper to use your own CA certificates to generate and sign server and client
certificates used for site linking, you can simply create the following structure under
the namespace home of your choice, for example:

```shell
${HOME}/.local/share/skupper/namespaces/default/input/certificates/
└── ca
    └── skupper-site-ca
        ├── ca.crt
        ├── tls.crt
        └── tls.key
```

With that, if you bootstrap a site to run in the default namespace, the CA certificates above will be
used to sign the server and client certificates for site linking for each provided `RouterAccess`.

Note that if a CA is provided at the time a site bundle is being installed, it will be detected,
but it won't be used unless the respective namespace is re-initialized. That is because when a bundle
is being installed, it will simply copy certificates provided by the user to be used internally, but
no new certificate will be signed during a site bundle installation.

## Server and Client certificates

Server and client certificates can be provided whenever your site definition contains at least
one `RouterAccess` (resource).

The expected directory names for the server and client certificates, is determined based on the
values of `RouterAccess.spec.tlsCredentials` (optional field), or `RouterAccess.name` (default).

Supposing the value of `RouterAccess.spec.tlsCredentials` or `RouterAccess.name` (when the tlsCredentials
field is omitted) is `my-router-access`, then the following structure, for server and client certificates,
must be provided under the namespace home of your choice, for example:

```shell
${HOME}/.local/share/skupper/namespaces/default/input/certificates/
├── client
│   └── client-my-router-access
│       ├── ca.crt
│       ├── tls.crt
│       └── tls.key
└── server
    └── my-router-access
        ├── ca.crt
        ├── tls.crt
        └── tls.key
```

At bootstrap or bundle installation times, you should see a message saying that the
user provided server and client certificates have been found.

As an example, inspecting the subject alternative names of the provided server certificate above,
and supposing it is valid for the following domain name:

```shell
X509v3 Subject Alternative Name: 
    DNS:my.local.server.com
```

If the following domain name is not defined as being the `spec.bindHost` or as part of the
`spec.subjectAlternativeNames` list of the `RouterAccess` resource, Skupper will also create a static
link that uses `my.local.server.com` as the target endpoint at:

```shell
$HOME/.local/share/skupper/namespaces/default/runtime/link/link-my-router-access-my.local.server.com.yaml
```

If the respective server certificates are defined at bundle installation time, Skupper will also inspect
the subject alternative names of the public server certificate and create the static links for each domain
name and ip address found, only if the `openssl` binary is available.

It is important that the client certificate is also provided, as all static links will be updated
to use the provided client credentials.
