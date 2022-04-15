
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


  Scenario: No policy, no services

   Given a cluster without any policies
    When trying to create a service
    Then it fails

  Scenario:

   Given a policy that allows services 'hello-world-.*' on all namespaces
    When trying to create a service named hello-world-backend, and one called hello-world-fronted
    Then both work
    When adding a policy that allows services 'hello-world-backend' and one that allows 'hello-world-frontend' on all namespaces
     and removing the policy that allowed 'hello-world-.*'
    Then the services are unaffected
    When changing these policies to apply only to the namespaces where the services are respectivelly deployed
    Then the services are unaffected, but they disappear from the list on the other namespace
    When adding both specific services (hello-world-{backend,fronted}) to the list on both namespace-specific policies, they reappear
    When Removing one of the policies and changing the other policy to point to a non-existent namespace
    Then both services are removed
     and trying to create new services fail
    When reinstating the policy that allows 'hello-world-.*' on all namespaces
    Then the services are not recreated

  Scenario: inversed
   Given a policy that allows service 'hello-world-frontend' on prv
    When trying to create service hello-world-frontend on pub
    Then it fails
    When changing for a policy that allows hello-world-frontend on all namespaces
     and successfully creating the service hello-world-frontend on pub
     and changing that policy to allow hello-world-front-end on prv
    Then the service is removed

  Scenario: Service binding

    There is nothing specific to service binding on the policies feature.  So, we just run the
    basic create/bind/unbind/delete hello world cycle, making changes to the policies in between
    each step, just to make sure the policies are not improperly removed

   Given a policy that allows .*-frontend on pub
     and one that allows .*-backend on prv
    When creating both services
    Then both are created, but show only on their respective namespaces
    When changing both policies for namespace "*"
     and binding the skupper services to their respective k8s services
    Then the binding works
     and both services are listed on both namespaces
    When changing the first policy to allow services '.*-.*'
     and removing the second policy
    Then the services and their listings are not affected
    When unbinding the services
    Then it works
    Continue
    

  Scenario: Annotation.  TODO
