e2e.test.rescue
=========

A role to invoke the must-gather container if any test task fails
It will collect useful cluster information and will save it for further use.


Requirements
------------

This role requires the oc command installed

Role Variables
--------------

The variables to run the must-gather container can be set in the role defaults.

Dependencies
------------

None

Example Playbook
----------------

  # Run Must-Gather on any failure
  rescue:
    ansible.builtin.include_role:
      name: e2e.tests.rescue

License
-------

Apache-2.0


Author Information
------------------

The Skupper team