
**Attention**: These tests apply and remove the Policy CRD and associated 
Policies as CRs.  Those have cluster-wise effect.  For that reason, they cannot
be run in parallel with any other tests, lest those other tests will fail.

# Specifications

* [Issue #655 - Policies for network and services](https://github.com/skupperproject/skupper/issues/655)
* [PR #668 - Policies for network, services and gateway](https://github.com/skupperproject/skupper/pull/668)
* [PR #701 - Site controller fixes permissions issues when policy is enabled](https://github.com/skupperproject/skupper/pull/701)
* [PR #703 - Fixes namespace label expression for policies](https://github.com/skupperproject/skupper/pull/703)
* [PR #705 - Policy to be considered enabled only by the presence of the CRD](https://github.com/skupperproject/skupper/pull/705)

# Note on files

Under the `features` directory there are files written in Gherkin, but not
read by any actual Gherkin system (behave or cucumber).

Instead, they're just a standardized way to write test cases before the actual
code can be written.

Note, however, that these files are incomplete and do not reflect the current
set of tests.  They were useful for the initial design of the tests, but were
neve completed.  See this [discussion](https://github.com/skupperproject/skupper/pull/784#discussion_r901791250)
for details.

# General questions

# Priorities

* A cluster without the CRD or with an all-allowing policy should behave like 0.8 

  (this will be not be the focus of these tests.  Instead, the main tests will
  be run in such clusters, providing for this coverage)

* Policy has teeth: anything that is not allowed should not be accessible

* Gateway, annotation testing are lower priority

For later

* An upgrade from 0.8 without CRD should also continue behaving like 0.8
* An upgrade from 0.8 with CRD pre-installed and an all-allowing policy
  should also continue behaving like 0.8

# Assumptions

* Invalid types on policy definition are taken care of by Kubernetes (eg setting
  a boolean flag as string, or a string list as a number), and will not be tested.

  If the user tries to patch or edit an existing path and enters a value that is
  invalid per CRD, Kubernetes also detects it and cancels the transaction (tested).

  That's for CRD syntax; semantics still need tested.

* Update testing is not on scope for now (it may be added to update-specific testing
  or added here in the future)

* API testing is not part of this test package as of now.

* Same for console testing

# Test factors

## The actual effects of policy items

Remember that policy items can be removed or commented out, and
that behaves as if they were false or empty.

### Positive: when a new policy allows an item

* allowIncomingLinks
  * token creation (including via console)
  * link creation (including via console)
  * gateway creation
  * if a link was previously brought down by policy, it should come back up
* AllowedOutgoingLinksHostnames (check FQDN and IPV4/6)
  * Outgoing link creation
  * if a link was previously brought down by policy, it should come back up
  * Note that testing FQDNs may be challenging on non-OCP clusters.  Start with
    IP, come back to FQDN and different Ingress options later.
* AllowedExposedResources (strict type/name; no regex)
  * Resource exposing
  * Resource exposing using annotations
  * Service binding
  * Resources unbound by policy are not re-bound when allowed again to
* AllowedServices
  * Make external services available
  * Skupper service creation
  * Annotation of services cause service exposing
  * Services removed by policy are not re-created when allowed again to
    * Even for skupper services created by annotation.  For those to be recreated, their 
      Kubernetes definitions need to be updated (so there is a trigger for Skupper to notice 
      the annotation), or the service-controller restarted
  * But remote services that were filtered out show up again

### Negative: when the last allowing policy is removed

(or when a CRD is added with no policies)

* allowIncomingLinks
  * stop working
    * token creation (including via console)
    * link establishment (including via console)
      * create token, remove allow, try to create link
      * links will be created, but in inactive state
    * gateway creation (moot, as no link)
  * Disconnect and disable
    * existing links
    * existing gateways
  * Existing tokens are not touched
* AllowedOutgoingLinksHostnames
  * outgoing link creation fails
  * existing links are disconnected and disabled
* AllowedExposedResources
  * binding of new resources fail
  * unbinding of resources (anything different about annotated?)
* AllowedServices
  * removal of local services (including exposed by annotation)
    * note that it is the Skupper service that is removed.  The original
      Kubernetes service is left intact, annotation and all
  * Make external services unavailable (on service status, they are listed but show
    as not authorized)
  

### Alternating

For some existing resources, when they are disallowed, they're removed for good.
Of others, however, they're only disabled.  Check that behavior by allowing and
disallowing the policy items a few times.

## The assynchronous nature of the policy engine

The policy engine works in a declarative manner: the service controllers in
each namespace monitor for policy changes and recalculate the local policy when
they change.

The testing needs to take that into account, and confirm that any pending
changes have been done to the tested namespace, lest it will report many
false positives and false negatives.

The GET checks from the PolicyTestSteps can be used to make sure the policy
engine has incorporated a CR change before proceeding with a state-changing
action.

## namespace selection

Namespace selection is on a list of regular expressions that match namespaces,
or label selectors (any tokens with a '=' in the middle), plus the special `"*"`
notation, representing any namespace.

*Note*: regexes are not anchored, so an item of `"."` matches any namespaces.
The same is true for all other policy settings that take a regex (services, 
outgoing hostnames).

A policy CR which points only to non-existing namespaces, or that has an empty
list for the namespace setting are in practice no-ops: they apply to no
namespaces.  For that reason, no effort has been done to test for invalid
namespace names (ie, with invalid characters or formats).

If any of the items on a list applies to a namespace, then the policy applies
to that namespace.

* `\*`
* regex
* label

The `Namespaces` selection works on an `OR` list, so besides single items,
it will be important to check that any lists work as expected.

Of course, one needs to make sure that policies that apply only to other 
namespaces make no changes on a given namespace, and that changes specific
to a namespace do not affect others.

Note that label selection items are the only place on the policy system that
behave with an `AND` nature:  all selectors in a given selector string must
apply to a namespace for that item to apply.  That, however, is [handled by
Kubernetes](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors).

* test multiple labels in a single item (AND-behavior)
* test single labels in multiple items (OR-behavior)

## the additive nature of policies

Policy items are disabled by default; a policy where everything is set to deny
is a no-op: it won't actually disable any of its items.

* Test no-op

Any policies that enable an item for a namespace are definitive, in the sense
that adding other policies that deny the item for the same namespace will have
no effect whatsoever.  Policy items will only be disallowed when all policies
that allow them and apply to the namespace are removed.

* Test two allowing policies, remove one and see what happens
* Then remove the other and ensure it is now disallowed

The actual policy in effect for a given namespace will be the merging of all
policies that apply to that namespace, with the following behavior:

* Boolean policy items behave as `OR`: any policies allowing and the item
  will be allowed
* List policy items behave as merge:  the resulting list will be the union
  of all the lists present on the policies that apply for the namespace

Note that policy items of type list also need to be 'activated': a resulting
policy with an empty `AllowedOutgoingLinksHostnames`, for example, will not
allow any outgoing connections (until specific hostnames or `"*"` are listed 
on it).


## Addition and removal of the CRD

Removal of CRDs also remove their CRs.  That means that the policies will be
removed.  That's Kubernetes' work, but we need to check side effects, if any.

A cluster without the CRD should behave like 0.8 (ie, policies play no role).

We don't need to test specifically for clusters without CRDs (the main tests
running without it will already cover that case).  However, we do need to 
check for the side-effects of CRD removal.

Addition of the CRD, also, has several side-effects.  More specifically,
links are dropped and services removed.

## Addition and removal of policies

Include editing of policies: does the policy engine recognize when a policy
has been changed in place, as opposed to removed and added?

Test for side-effects of removal of policies.

### Verification

For most tests, the verification will be done through attempts to run the
affected cli commands *and* access to the `get` API. However, that may not be
the case in some situations:

* When verifying that a change did not affect something it was not intended
  * To confirm service creation would work, use only `get`, if the actual
    service creation is not in the test's interests
  * To confirm it would fail, use the cli, as it should fail anyway
* For performance reasons.  Perhaps make this environment-configurable

## Annotation-based skupper enablement

Remember that skupper services can be created by adding annotations to services.

So, besides cli testing, make sure to test with annotations (low priority).

## Others

* test via operator + config map
* test with non-admin skupper init
  * Discussion around cluster role and policy being enabled
  * If the Service Account is not created and given the role binding, should
    the policy be enabled or disabled?
* skupper networks status (brand new tests)

# Helpers

Describe features of the product that may help writing test cases.

## service controller command `get`

* `get events`
* `get policies`

## service controler pod logs

## service sync

It happens every 5 seconds.  

It should not be a concern for the testing, as the CLI testing infra, in
special the `status` methods, works by retrying the command until it prints
the expected message.


# TODO


## Trying to circumvent controls

* Make changes directly on the configuration maps
* Other changes that can be done by a namespace admin that is not a cluster admin

## Pod restarts

To make sure that any changes were persisted.  Idea is to have it by environment
variable, or a new TestRunner.  In-between each task, restart the pods

# Other checks

* Running a non-policy skupper binary against a policy-enabled service 
  controller (when I did that by mistake, cpu usage from the skupper
  pods went to the roof)

* bypass the skupper binary: make direct API calls.

# Suggestions



