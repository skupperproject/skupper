# 9000. Upgrade skupper sites from v1 to v2

Date: 2024-11-04

## Status

Proposed

## Context

We need to document how customers will upgrade v1 skupper sites to
v2 skupper sites.

(TODO: refer to some other doc that explains: v2 configuration via CRs.)

## High level decisions

High level decisions to document further:

1. The skupper community will define the recommended procedure
   to upgrade v1 skupper sites to v2 skupper sites.

2. The skupper community will produce a document that details
   the steps to take a v1 site and upgrade to a v2 site.

   The document could cover the following:
   * document the v2 CRs that need to be created.
   * if v1 site has 2 relicas in the router deployment, then configure
     HA in v2.
   * if v1 site has specific CPU/memory limits on v1 resources, then
     in v2 edit deployment yaml. 

3. A tool will be created to create skupper v2 CRs for given
   skupper v1 sites. 

   Details include:
     * In order to read the skupper v1 configuration, the tool
       will be provided online access to the skupper v1 cluster.

4. On upgrade, a skupper v2 site will be installed in the same namespace
   where the skupper v1 site was installed.

   Note that skupper v1 and skupper v2 sites cannot be installed in
   the same namespace at the same time.

   As a result, a skupper v1 site must be uninstalled in a namespace
   before a skupper v2 site can be installed in that same namespace.

   Reasons that skupper v1 and skupper v2 sites cannot be installed in
   same namespace include:

   * configmap name collision: skupper-network-status

   * service name collision.  Example: listener service name.

   * secret name collision.  For example:

```
       $ kubectl -n west-v1 get secret
       NAME                                   TYPE                DATA   AGE
       skupper-local-ca                       kubernetes.io/tls   2      5d
       skupper-local-server                   kubernetes.io/tls   3      5d
       . . .
       skupper-site-ca                        kubernetes.io/tls   2      5d
       skupper-site-server                    kubernetes.io/tls   3      5d
```

4. On upgrade to v2, links should be reestablished between sites.

   Up upgrade to v2, links can be reestablished as follows:
     * create AccessGrant CRs in the target v2 site
     * wait for the v2 Controller to populate the
       AccessGrant CR with: CA, code, URL   
     * edit an AccessToken CR with: CA, code, URL
     * create AccessToken CRs in the originating v2 site

   Discuss: 
     * Are links transferable from a v1 skupper site to a v2 skupper site?
     * Is there a low friction way to create v2 links for the user?

### Upgrade steps: delete v1 site, create v2 site in same namespace

Upgrade steps include:

  1. Provide online access to all v1 sites to tool.

     * Tool reads configmap skupper-site for all sites.

       Tool keeps a map of site uid and site config.

     * Tool identifies configured links by: reading secret
       metadata type=connection-token.

     * Tool identifies service configuration by: reading configmap
       skupper-servivces.

  2. Tool generates CRs.

     CRs may be stored as yaml files.

     If upgrade tool is called programatically, CRs may be returned
     programatically as yaml strings.

     * Site CR

       For every v1 skupper-site configmap

     * AccessGrant CR

       For every v1 secret type=connection-token.

       Use generated-by annotation to identify target site.

       TODO: discuss: 1 access grant per v1 link?

     * AccessToken CR

       For every v1 secret type=connection-token.

       Note: credential fields are populated at a later step.

     * Listener CR

       For every skupper-services configmap entry.

       Create a Listener CR for each service port.

     * Connector CR

       For every skupper-serices configmap entry with targets populated.

       Create a Connector CR for each target entry and for each target port.

  3. User deletes v1 skupper sites.

  4. User installs v2 CRDs in cluster.
     This step requires cluster permissions.

  5. User creates v2 skupper controller deployment.
     User starts v2 skupper controller.

     * User may start 1 controller per cluster.
     * User may start 1 controller per namespace.

  6. User applies v2 yaml files:

     * Site CR
     * Access Grant CRs
     * Listener CRs
     * Connector CRs

  7. User waits for v2 Controller to populate AccessGrant CRs with
     credential info: CA, code, URL.

  8. User edits v2 AccessToken CRs to include AccessGrant credential
     info: CA, code, URL.
  
## Open upgrade questions

1. How will tool read v1 skupper state: online (kube access) vs 
   offline (debug dump).

2. Identify different upgrade options:

   1. delete v1 site, create v2 site in same namespace.

   2. create v2 site in same namespace.

      v1 and v2 run in the same namespace for some time period.

   3. create v2 site in separate namespace.  

      v1 and v2 sites both run in cluster for some time period.

      Evaluate: Would this option allow the user to verify v2 is working
      well before deleting v1?

3. If an upgrade tool generates v2 CRs, should the tool
   allow the user to review and customize the CRs?

4. If a skupper v1 site has multiple skupper-router replicas, should
   HA be configured in v2 site by default? 

5. If v1 site has policy in place, what upgrade steps are
   needed in the v2 site?  How would this change RBAC in v2?
   Is it adequate to just document the steps to configure
   RBAC in v2?

6. If he upgrade detects service-sync enabled, should the
   upgrade tool create listeners at all v2 sites?

7. During upgrade to v2, should it be possible for the user to
   rollback to v1?

   For example, if the upgrade to v2 has an issue, can user
   rollback to v1?

8. Discuss how to upgrade v1 services with multiple ports.
   Create multiple listener or connector CRs, 1 CR per port?
   Decide on CR name: backend-tcp-8080?

9. Who is responsible for deleting the v1 site?
   Is user responsible for deleting the v1 site?

10. If the collector is enabled in the v1 site, should
   upgrade tool start the collector deployment in v2?
   Or should upgrade document explain that user should
   start the collector deployment in v2?

11. If upgrade detects cpu / memory limits, how to apply
   these settings during upgrade to v2?

12. Should the upgrade process restore annotated services?

    If you had an annoated service in v1, at some point this service 
    should be unnannotated in order to work as expected in v2.
 
    Will service be unnannotated by deleting the v1 site?

    Or will service be unnannotated by some other step?


## Fixed vs immutable resources during upgrade

Fixed resources include:

1. Namespace.  

Mutable resources include:

2. Annotated services.

## Options

## Decision

## Consequences

## HCM open questions

HCM open questions include:

## TODOs

