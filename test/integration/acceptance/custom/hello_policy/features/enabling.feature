
Feature: enabling and disabling policy

  There are three ways to enable and disable the whole of the policies:

  1 - install or remove the CRD and namespace permission to read policies
  2 - with a CRD installed, remove all policies or add an all-allowing policy (or a set thereof)
  3 - Remove the permission to read policies

  Do these two actions cause the same side effects?

  This test verifies these activations and deactivations as whole:

  1 - Starts with two skupper networks, no CRD (may be brand new or update)
  2 - Run the hello world, just to confirm everything is in order (it may be part of setup)
  3 - Apply the CRD
  4 - Verify that all elements affected by policy were brought down or removed
  5 - Verify that creation of new elements affected by policy does not work
  6 - Apply an all-allowing policy
  7 - Verify that links come back up, and that external services show up again (on two cluster env)
  8 - Run hello world again, ensure it all works
  9 - Remove that policy
  4 - Verify that all elements affected by policy were brought down or removed
  5 - Verify that creation of new elements affected by policy does not work
 10 - Remove the CRD
 11 - Verify that links come back up, and that external services show up again (on two cluster env)
 12 - Run hello world again, ensure it all works

  Other options
  - Add no-op policy after 6, confirm no changes

  Test factors:
  - Actual effect of policy items
  - Addition or removal of CRD
  - Addition or removal of policy






# TODO: Rewrite below

  Scenario: add and remove CRD, with hello_world
    Given a cluster with no CRD and no policies
     When hello_world is run
     Then everything works, exactly like hello_world     # reuse hello_world test table?
     When the CRD is installed on private
     Then the link drops on private
      and the existing skupper services are deleted on private
      and the private skupper services disappear from public
      and service creation and exposing fail on private
      and service binding, unbinding, deletion, unexposing are impossible on private, as there is no service
      and service creation, binding, unbinding, exposing and unexposing work on public, but won't show up on private
     When the CRD is removed from private
     Then the link comes back up on private
      and the deleted services are still missing on private
      and hello_world works again on private

  Scenario: remove and re-add an all-allowing policy
    Given a cluster with CRD and an all-allowing policy for all domains
     Then steps should work exactly like the one above?  Will service be removed, in particular?

