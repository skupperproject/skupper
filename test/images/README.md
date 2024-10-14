# Images for external test dependencies

The Makefile on this directory allows for the manipulation of container images
pertaining to external test dependencies, such as Mongo or quiver.

It contains three operations: the first exists to avoid the Dockerhub pull
limit, and also to allow for testing on disconnected clusters.  It simply
copies the images from their original locations into our Quay repo.

The second operation was created in response to an image used by our tests that
stopped getting updates, while the actual project continued to evolve.  It
builds images from Containerfiles and pushes them into our Quay repo.

Finally, the third one also allows for the running of tests on disconnected
clusters, which cannot access the original locations of these dependencies to
download them (as they have no external Internet access).  The operation allows
for copying the list of images from our Quay repo into another repo.

This allows for the test dependencies to be copied to the disconnected
cluster's local repository, from where the cluster can access them.  Additional
configuration will be required on the cluster (such as creating an MCO to
re-route the image pull requests), but that's out of the scope of this
document.

Note that this is not a full repo copy; it's restricted to the list of images
contained on the Makefile.  Skupper images (such as the router or controller)
are not copied by this operation.

Note also that it is not a simple copy.  As Skupper can run on some older
Kubernetes that do not support the OCI format, there are some transformations
done during the copy as well.

See the Makefile contents for information on how to execute the different
operations.


# skupper-test image

Note that the `skupper-test` image used by Skupper integration is part of
Skupper's own test code, so it's built by the main Makefile at ../.., and not
here.
