# TCP Echo Half-Closed Connection Test

This README provides an overview of the "TCP Echo Test" Ansible playbook for testing Skupper connectivity and simulating a half-closed connection.

## Overview

The TCP Echo Test demonstrates a basic Skupper setup between two Kubernetes clusters (named "west" and "east"). It deploys a TCP Echo backend in the east cluster, exposes it via Skupper on port 9090, and connects to it from the west cluster using netcat.

Specifically, it utilizes a shell command `(cat payload.txt; sleep 10) | nc tcp-go-echo 9090` to simulate a half-closed connection, allowing developers to test and verify solutions for file descriptor leaks in the Skupper router.

## Running the test

To run this specific test, execute the following command from the `tests` directory:

```bash
make test TEST=tcp-echo
```
