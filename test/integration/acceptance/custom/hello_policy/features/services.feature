
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

   # no-policy-service-creation-fails
   Given a cluster without any policies
    When trying to create a service
    Then it fails

  Scenario:

   # all-hello-world-works
   Given a policy that allows services 'hello-world-.*' on all namespaces
    When trying to create a service named hello-world-backend, and one called hello-world-fronted
    Then both work
    # add-specific-policies--remove-general--no-changes
    When adding a policy that allows services 'hello-world-backend' and one that allows 'hello-world-frontend' on all namespaces
     and removing the policy that allowed 'hello-world-.*'
    Then the services are unaffected
    # make-policies-specific-to-namespace
    When changing these policies to apply only to the namespaces where the services are respectivelly deployed
    Then the services are unaffected, but they disappear from the list on the other namespace
    # policies-list-both-services
    When adding both specific services (hello-world-{backend,fronted}) to the list on both namespace-specific policies
    Then they reappear
    # policy-removals
    When Removing one of the policies and changing the other policy to point to a non-existent namespace
    Then both services are removed
     and trying to create new services fail
    # reinstating-and-gone
    When reinstating the policy that allows 'hello-world-.*' on all namespaces
    Then the services are not recreated
     and trying to create services with the same names works
    # allow-but-not-this
    When changing that single policy to allow 'non-existent-service'
    Then the services are removed

  Scenario: Service binding

    For Policy, service binding is controlled by AllowedExposedResources, and
    tested on the resources test file.

    Here, we just run the basic create/bind/unbind/delete hello world cycle,
    making changes to the policies in between each step, just to make sure the
    bindings are not improperly removed (or improperly preserved, in the case
    a service was removed by policy)

   # init-for-binding
   Given a policy that allows .*-frontend on pub
     and one that allows .*-backend on prv
    When creating both services
    Then both are created, but show only on their respective namespaces
    # first-binding
    When binding the skupper services to their respective k8s services
    Then the binding works
    # show-on-both
    When changing both policies for namespace "*"
    Then both services are listed on both namespaces
    # reorganize--no-effect
    When changing the first policy to allow services '.*-.*'
     and removing the second policy
    Then the services and their listings are not affected
    # unbind
    When unbinding the services
    Then it works
    # re-bind
    When Adding a policy that allows .*-backend on prv
     and rebinding the services
    Then it works
    # partial-allow
    When removing the first policy
    Then the front-end skupper service is removed
     and the backend skupper service shows as not-allowed on pub
    # re-add--re-create--not-bound
    When re-adding a policy that allows all services on all namespaces
     and recreating the front-end service
    Then both services appear on both namespaces
     But the front-end service is not bound
     
    Continue
    

  Scenario: inversed
   Given a policy that allows service 'hello-world-frontend' on prv
    When trying to create service hello-world-frontend on pub
    Then it fails
    When changing for a policy that allows hello-world-frontend on all namespaces
     and successfully creating the service hello-world-frontend on pub
     and changing that policy to allow hello-world-front-end on prv
    Then the service is removed

  Scenario: Annotation.  TODO
