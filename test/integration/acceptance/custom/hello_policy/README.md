
Under the `features` directory there are files are written in Gherkin, but not
read by any actual Gherkin system (behave or cucumber).

Instead, they're just a standardized way to write test cases before the actual
code can be written.

# General questions

"*It will also provide a Rest API for policy validation. This API can be used by the
CLI, through the get command, or by the Console in the future.*".  Is there
an API to be tested?  So far, work was centered on cli.  Or is that just the `get`
command on the service controller?

Allowed services: only local, or remote, too?  Soo, if service 'foo' is not allowed,
it means that a service 'foo' cannot be exported locally, nor can it be consumed
from elsewhere?

# Priorities

* A cluster without the CRD or with an all-allowing policy  should behave like 0.8
* Policy has teeth: anything that is disallowed should not be accessible
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
* AllowedExposedResources (strict type/name; no regex)
  * Resource exposing
  * Resource exposing using annotations
  * Resources unbound by policy are not re-bound when allowed again to
* AllowedServices
  * Skupper service creation
  * Annotation of services cause service exposing
  * Make external services available
  * Services removed by policy are not re-created when allowed again to
  * But remote services that were filtered out show up again (?)

### Negative: when the last allowing policy is removed

(or when a CRD is added with no policies)

* allowIncomingLinks
  * stop working
    * token creation (including via console)
    * link creation (moot, as no token) (including via console)
      * actually: create token, remove allow, try to create link
    * gateway creation (moot, as no link)
  * removals
    * existing tokens?
    * existing links
    * existing gateways
* AllowedOutgoingLinksHostnames
  * outgoing link creation fails
  * removal of existing links
* AllowedExposedResources
  * binding of new resources fail
  * unbinding of resources (anything different about annotated?)
* AllowedServices
  * removal of local services (including exoposed by annotation)
  * Make external services unavailable
  

### Alternating

For some existing resources, when they are disallowed, they're removed for good.
Of others, however, they're only disabled.  Check that behavior by allowing and
disallowing the policy items a few times.

## The assynchronous nature of the policy engine

The policy engine works in a declarative manner: when policies are added or 
removed, it recalculates the policies and apply any changes to the individual
namespaces.   (or, actually: the service controllers in each namespace monitor
for policy changes and recalculate the local policy when they change?)

The testing needs to take that into account, and confirm that any pending
changes have been done to the tested namespace, lest it will report many
false positives and false negatives.

* Detect policy engine conclusion of work

## namespace selection

Namespace selection is on a list of actual namespaces, regexes thereof, or 
label selectors (any tokens with a '=' in the middle).

TODO:  Confirm this.  "\*" is not a valid regex, still.  Also, does it test it
twice?  One with regex, one with normal?  If not, how does it treat the dots?

*Question*: Where are regexes anchored?  If everything is a regex and they're
not anchored, then an item of '.' would also match everything

* check and document (link) Kubernetes allowed characters for namespaces
* should invalid names make the policy invalid?

If any of the items on a list applies to a namespace, then the policy applies
to that namespace.

* "\*"
* specific namespace
* regex
* label

The `Namespaces` selection works on an `OR` list, so besides single items,
it will be important to check that any lists work as expected.

Question: what would `Namespaces: []` stand for?  Invalid policy that applies
to no namespaces at all?

Of course, one needs to make sure that policies that apply only to other 
namespaces make no changes on a given namespace, and that changes specific
to a namespace do not affect others.

Note that label selection is the only place on the policy system that behaves
with an `AND` nature:  all given labels (in an item in the list) must apply to
a namespace for that label selection to apply.  This should be Kubernetes work
to ensure, but we test anyway

* test multiple labels in a single item
* test single labels in multiple items

## the additive nature of policies

Policies are disabled by default; a policy where everything is set to deny 
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
allow any outgoing connections (until specific hostnames or "\*" are listed 
on it.


## Addition and removal of the CRD

Removal of CRDs also remove their CRs.  That means that the policies will be
removed.  That's Kubernetes' work, but we need to check side effects, if any.

A cluster without the CRD should behave like 0.8 (ie, policies play no role).

Question: should we test specifically for cluster without CRD?  Or leave the
main tests to do that?

Addition of the CRD, however, has several side-effects.  More specifically,
links are dropped and services removed.

## Addition and removal of policies

Include editing of policies: does the policy engine recognize when a policy
has been changed in place, as opposed to removed and added?

Test for side-effects of removal of policies.

## Test steps

The tests have the following four identifiable phases:

* Background - basic environment configuration, shared by multiple tests
* Preparation ("Given") - changes to that basic environment that set it
  to the specific state required by the test.
* Modification ("When") - execution of the actual feature being tested
* Verification ("Then") - confirmation that the feature works ok

### Backgrounds

Some tests need to be repeated in different backgrounds, to ensure they
work the same, while others will require a specific background to produce
a specific result.

Here 'background' means simply the state of the cluster and namespaces at the
start of the test.

* current vs 0.8 (or 'previous'.  Make it configurable)
* pre-existing skupper (just init)?
* pre-existing skupper network?
* pre-existing CRD or no?
* pre-existing policy or no?  Permissive or not?

Note the list above can generate new backgrounds as combinations (eg 0.8 with
CRD installed before update)

A special background needs provided for semi-automated testing: 'do not touch'.
It is simply a no-op background.  The tester can prepare the environment
(background) to what they need before running a specific test with it, so the
actual background is whatever the tester prepared manually, but the test still
runs from the code.

Question: what cluster-wide changes does skupper do?  How to make sure they're
fully removed?  Service account; anything else?

Idea: for cluster wide modifications, run the preparation for a set of tests,
then a single modification, then the verification for all of them.  This might
save some time on the test.

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

Remember that skupper services can be created by adding annotations to services
(is that so?  add link to documentation).

So, besides cli testing, make sure to test with annotations.

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

It happens every 30 (?) seconds.  

TODO: Check that and how to confirm it's been run


# TODO


## single or multiple clusters 

Should a second set of tests be written for two-cluster configurations, or
should the tests change their behavior for that configuration?

All tests should be able to run in a single-cluster or two-cluster testing
configuration, and the results should be expected to be different.  For
example, if a CRD is applied on the private cluster and then removed, the link
from private to pub will drop, but not reconnect on removal (check whether this
is expected behavior — it is current behavior), but in any case things can be
assymetrical.

# Other checks

* Running a non-policy skupper binary against a policy-enabled service 
  controller (when I did that by mistake, cpu usage from the skupper
  pods went to the roof)

* bypass the skupper binary: make direct API calls.

# Suggestions

* On the CRD that will be made available to clients, add a comment with
  capital letters WARNING that installing the CRD enables the Policy
  immediatelly, that can remove services and etc.
 
* `get policies status`: on or off and list of policies



