# User provided certificates

Users can provide their own certificates to be used with Skupper V2 in non kube sites
during the bootstrap of a local site, when preparing a site bundle and even while installing
a site bundle at a remote machine.

## Certificate Authorities (CAs)

Certificate Authorities (CAs) can be provided at the time a local site is initialized
or a site bundle is being prepared. After a local site is created or a bundle is produced,
CAs won't be used to sign certificates, unless an active namespace is re-initialized.

As an example, if you want Skupper to use your own CA certificates to generate and sign server
and client certificates used for site linking, you can simply create the following structure under
the namespace home of your choice, for example:

```shell
${HOME}/.local/share/skupper/namespaces/default/input/certificates/
└── ca
    └── skupper-site-ca
        ├── ca.crt
        ├── tls.crt
        └── tls.key
```

With that if you bootstrap a site to run in the default namespace, the CA certificates above will be
used to generate the server and client certificates for site linking.

## Server and Client certificates (for site linking)

Server and client certificates can also be provided to help with site linking.

When a local site is initialized, a bundle is being prepared or installed, Skupper will
inspect the Subject Alternative Names (SANs) from the provided server certificate, and
it will generate a static link for each of the entries, so that they can be distributed
to the appropriate client sites for site linking.

The expected directory names for the server and client certificates, is determined based on the
values of `RouterAccess.spec.tlsCredentials` (optional field), or `RouterAccess.name` (default).

Supposing the value of `RouterAccess.spec.tlsCredentials` or `RouterAccess.name` (when the tlsCredentials
field is omitted) is `my-router-access`, then the following structure, for server and client certificates,
must be provided:

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
