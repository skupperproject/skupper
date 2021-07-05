# Claims CLI integration test

This CLI integration test validates token management using the
`skupper` (binary) CLI. A few scenarios are executed to validate
different usages of `skupper token create` command.

# Scenarios

## Token claim creation (default)

Executes `token create` command, with no extra flags.

Steps:
* Generates a token claim on public namespace
* Asserts that:
  * Claim record has expiration >= 15m
  * Claims-remaining = 1
  * Password has been generated
  * Generated type is: token-claim
* Connect private with public namespaces
* Expect sites connected
* Deletes the link from private namespace
* Attempt to re-create the link using the same token-claim
* Assert connection failed and that token can be used only once
* Deletes the link
  
## Token certificate create

Executes `token create --token-type cert` and validate it
creates a `connection-token` that can be reused as much
as needed, without limits or expiration.

Steps:
* Generates a token certificate on public namespace
* Asserts that:
  * Generated type is: connection-token
* Connects private with public namespaces
* Expects sites connected
* Deletes the link from private namespace
* Attempts to re-create the link using the same connection token
* Expects sites connected
* Deletes link

## Token creation using all flags

Executes `token create` command, with all extra flags.
Test will set all flags:
* --expiry
* --name
* --password
* --token-type
* --uses=2

Steps:
* Generates a token claim on public namespace
* Asserts that:
  * Expiry is defined
  * Claim name is defined
  * Password is defined
  * Generated type is: token-claim
  * Number of uses is defined
* Links private with public namespaces
* Deletes the link
* Recreate the private link to public
* Assert that connection worked as claim can be used twice
* Deletes the link

## Token expiry

Executes `token create` command, using `--expiry 1s`, to assert
that token claim will be expired at the moment it is used.

* Generates a token claim on public namespace with a 1s expiration
* Asserts that:
  * Claim expiration is 10s
* Delay 15s to ensure claim is expired
* Connect private with public namespaces
* Assert connection failed because token-claim is expired
* Deletes the link
