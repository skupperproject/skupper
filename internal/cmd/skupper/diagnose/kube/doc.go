/*
Package kube provides Kubernetes-specific diagnostics.

Each diagnostic is implemented in a function which is given a KubeClient and a status reporter.
The status reporter should be used to report warnings only;
outright errors and success are indicated respectively by returning an error and returning nil,
and are handled by the caller.
Each diagnostic is described by an instance of KubeDiagnose, constructed using newKubeDiagnoseCommand().
The name should be suitable as a Cobra command name, and the check description should work in a sentence of the form
"Checks that …". The check description is used to build command-line help and to provide status messages.
Diagnostics can have dependencies, which are other diagnostics.
These should be strong dependencies: when a given diagnostic is invoked, its dependencies will be invoked first,
and if any of them return an error, the entire chain will be aborted.
*/
package kube
