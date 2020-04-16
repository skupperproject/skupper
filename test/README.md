# Running tests

1. First you need to be authenticated into a cluster (with cluster-admin permissions)
1. Then you must have skupper (binary) in your PATH (preferrably using the one produced by make)
1. You must have ginkgo installed (go get -u github.com/onsi/ginkgo/ginkgo)
1. Go to the test directory: `cd test`
1. Then you can run: `ginkgo -v -r ./examples`

