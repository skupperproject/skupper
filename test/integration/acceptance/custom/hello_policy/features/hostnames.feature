
Feature: Skupper link limiting with AllowedOutgoingLinksHostnames

  Test factors: 

  - The actual effects of policy items
  - Addition and removal of policies

  Background:
    Given the public cluster has the policy CRD installed
      and the private cluster has the policy CRD installed
      and the public cluster has no policy CRs
      and the private cluster has no policy CRs
      and hello-world-frontend is deployed on pub
      and hello-world-backend is deployed on prv

  Scenario: No policy, no link

   # no-policy-service-creation-fails
   Given a cluster with an all-allowing policy, except for AllowedOutgoingLinksHostnames
    When trying to create a link
    Then it fails

  Scenario: 
   Given a cluster with an all-allowing policy
    # create-token-link
    When trying to create a link
    Then it works
     and we can register metadata.annotations.edge-host of its secret as $target
    # remove-tmp-policy-and-link
    When we remove all permissions on AllowedOutgoingLinksHostnames
    Then the link drops
     and we can also remove the link
    When we add a single AllowedOutgoingLinksHostnames containing ^$target$
    Then it comes up again
    When we set AllowedOutgoingLinksHostnames as $target, but only till the first dot, anchored on both sides
    Then it comes down
    When we remove the anchor at the end
    Then it comes up
    When we set AllowedOutgoingLinksHostnames as $target, but only from the last dot, anchored on both sides
    Then it comes down
    When we remove the anchor at the start
    Then it comes up
    When we change first and last characters of $target to be literal dots (\.)
    Then it goes comes down
    When we change first and last characters of $target to be simple dots (.)
    Then it comes up
    When we change every other character of $target to be a dash (-)
    Then it goes down
    When we change every other character of $target to be a dot (.)
    Then it comes up



