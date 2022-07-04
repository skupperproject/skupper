
# Move these to their own files


Feature: Policy calculation

  The effective policy for a namespace is based on all policies that
  apply to that domain, and it needs recalculated every time that
  a policy is changed.

  Here, the verifications are done only via `get policies`, to speed
  things up, whereas elsewhere both `get policies` and actual cli 
  commands are used.



  Test factors:

  - the additive nature of policies
  - addition and removal of policies

  Scenario: Add and remove a single policy
 
  # combinatorial
  Scenario: Add the same item in different policies with different namespace selectors; remove one by one

  Scenario: same as above, but for string lists


Feature: Individual policy items

  Test in detail the effects of each policy item

  - What is allowed while it is in effect
  - What changes when it comes into effect
    - Disabled or removed
    - Enabled and not readded

  Test factors: 

  - The actual effects of policy items
  - Addition and removal of policies
