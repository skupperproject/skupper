

Feature: AllowIncomingLink

  Test factors: 

  - The actual effects of policy items
  - Addition and removal of policies

  Background:
    Given the public cluster has the policy CRD installed
      and the private cluster has the policy CRD installed
      and the public cluster has no policy CRs
      and the private cluster has no policy CRs
 
  Scenario: empty-policy-fails-token-creation

     When trying to create a token
     Then the token creation fails
      and GET allowIncomingLinks == false
   
  Scenario: allowing-policy-allows-creation

    Given a policy that allows only IncomingLinks on the public namespace
      and a policy that allows all outgoing hosts for links on the private namespace
     When creating a token
      and using the token to create a link
     Then the creation works successfuly
      and GET allowIncomingLinks == true
     When removing the policy that allows IncomingLinks on public namespace
     Then the link goes down
      and GET allowIncomingLinks == false
     When re-enabling the policy that allows IncomingLinks on public namespace
     Then the link goes up
      and GET allowIncomingLinks == true
      and link removal works fine

  Scenario: previously-created-token

    Given a policy that allows only IncomingLinks on the public namespace
      and a policy that allows all outgoing hosts for links on the private namespace
      and creating a token
     When removing the policy that allows IncomingLinks on public namespace
      and using the token to create a link
     Then the link creation suceeds
      But the link is shown as inactive
      and GET allowIncomingLinks == false
      and link removal works fine
     When re-enabling the policy that allows IncomingLinks on public namespace
     Then the link comes up
      and GET allowIncomingLinks == true
      and link removal works fine

  Scenario: TODO-gateway

