
Feature: namespace selection

  A policy will be applied to a namespace only according to the 
  policy's `namespaces` field.

  All tests are done in a cluster which already has a CRD installed and
  no policies at the start of each test.  

  The tests are centered on namespace selection, so they only test one
  resource of each type: boolean or string list.

  For boolean, it tests token creation based on `allowIncomingLinks`.

  For string list, it tests `allowedServices`

  Note that an empty namespace list (`namespaces: []`) is a valid selection,
  which applies to no namespaces at all, making the policy a no-op.

  Test factors:
  - namespace selection
  - the additive nature of policies
  - addition and removal of policies

  Background:
    Given a cluster with CRD installed and no policies

  Scenario: Without a policy, token creation fails
     When skupper token create tmp/token.yaml
     Then the command fails


  # TODO: Should these simpler scenarios be left for unit testing?
  Scenario Outline: Single policy that allowIncomingLinks, single namespace selection, token creation
    Given a single policy that allowIncomingLinks on <namespace>
     When skupper token create tmp/token.yaml
     Then the command <status>
    Examples: namespaces
     | namespace                | status |
     | [ "*" ]                  | works  |
     | [ "this_namespace" ]     | works  |
     | [ "other_namespace" ]    | fails  | 
     | [ "regex_matching" ]     | works  |
     | [ "regex_not_matching" ] | fails  |
     | [ "app=this_app" ]       | works  |
     | [ "app=other_app" ]      | fails  |
     | [ "app=this_app,miss=1" ]| fails  | # AND behavior for label selection
     | [ "app=this_app,here=1"] | works  | # AND behavior for label selection

  # This could actually be combined with the one above, in a permutational manner.
  # From the generated permutation, if any has status 'works', the expected outcome should 
  # be works (OR).  From the 8 possibilities above, the permutation numbers:
  # len(1): 8
  # len(2): 8 * 7 (considering no dups and testing position in the list) = 56
  # len(3): 8! / (8-3)! = 336
  Scenario Outline: Single policy that allowIncomingLinks, multiple namespace selection, token creation
    Given a single policy that allowIncomingLinks on <namespaces>
     When skupper token create tmp/token.yaml
     Then the command <status>
    Examples: namespaces
     | namespaces                                       | status |
     | [ "*", "this_namespace" ]                        | works  |
     | [ "*", "other_namespace" ]                       | works  | 
     | [ "*", "other_namespace", "this_namespace" ]     | works  |
     | [ "other_namespace", "yet_another" ]             | fails  | 
     | [ "regex_matching", "other" ]                    | works  |
     | [ "regex_not_matching", "this" ]                 | works  |
     | [ "app=this_app", "other" ]                      | works  |
     | [ "app=other_app", "this" ]                      | fails  |
     | [ "app=this_app,miss=1" ]                        | fails  | # AND behavior for label selection

  # This one could also be combinatorial.  In this case, dups should naturally be allowed,
  # so the maths are simpler:
  # len(2): 8^2 = 64
  # len(3): 8^3 = 512.  This is a bit too much (64 possibly is, already), but we could go random here
  # The examples below are selected from that list, to go over some simple cases, and some
  # edge cases (for example, ensure that a more-specific does not override a more-broad
  Scenario Outline: Two policies, different values of allowIncomingLinks, single namespace selection, token creation
    Given a policy that allowIncomingLinks on <allow_namespace>
      and a policy that does not allowIncomingLinks on <no_allow_namespace>
     When skupper token create tmp/token.yaml
     Then the command <status>
    Examples: namespaces
     | allow_namespace          | no_allow_namespace       | status |
     | [ "this_namespace" ]     | [ "this_namespace" ]     | works  | # conflicting
     | [ "*" ]                  | [ "other_namespace" ]    | works  | # broad/specific
     | [ "other_namespace" ]    | [ "*" ]                  | works  | # specific/broad
     | [ "other_namespace" ]    | [ "other_namespace" ]    | fails  | 
#     | [ "regex_matching" ]     | [ "regex_matching" ]     | works  |
#     | [ "regex_not_matching" ] | [ "regex_not_matching" ] | fails  |
#     | [ "app=this_app" ]       | [ "app=this_app" ]       | works  |
#     | [ "app=other_app" ]      | [ "app=other_app" ]      | fails  |
#     | [ "app=this_app,miss=1" ]| [ "app=this_app,miss=1" ]| fails  | # AND behavior for label selection
#     TODO: continue examples (pick)

  # I only see this being done with combinatorial and random selection of combinations
  Scenario Outline: Multiple policies, different values of allowIncomingLinks, multiple namespace selection, token creation

  # Repeat all tests above with allowedServices, taking care that here it is append behavior
  Scenario Outline: allowedServices
