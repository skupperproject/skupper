

Feature: Skupper service creation and binding

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


  Scenario: 
   Given a cluster with an all-allowing policy
     and a skupper link established between two namespaces
    When trying to create a link
    Then it works
