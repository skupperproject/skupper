# This make file provides three operations:
#
# - Copy (replicate) a list of images (hardcoded below) to a main repository
#   (by default, quay.io/skupper), possibly modifying and reassembling them
# - Generate and push images to that same repository, based on Containerfiles
#   located on this directory
# - Copy both lists of images above (replicated and generated) to another,
#   configurable repository, along with other two lists, which are generally
#   used from their original locations ('other' and 'external').
#
# See the README.md for the rationale of each of them.
#
# The basic operation is to replicate or generate-and-push a single image into
# the MAIN_REPO:
#
#   make docker.io/svagi/nghttp2
#   make docker.io/nginxinc/nginx-unprivileged
#   make nghttp2
#
# The possible values are those listed on REPLICATED_IMAGES, REASSEMBLED_IMAGES
# and GENERATED_IMAGES.
#
# You can also only build an image (and not push it).  Just add _build to the
# image name listed in GENERATED_IMAGES:
#
#   make nghttp2_build
#
# Alternativelly, one can build/push or copy all generated, all replicated, all
# reassembled, or all of them:
#
#   make replicated_images
#   make reassembled_images
#   make generated_images
#   make everything
#
# Notice that the Makefile itself will not check whether the copy or build/push
# is necessary; the underlying tools (podman and skopeo) may do that, but they
# may still consume Dockerhub pull requests in the process.
#
# As with any Makefile, you can configure the execution by setting variables
# on the make invocation:
#
#   make MAIN_REPO=quay.io/dhashimo docker.io/library/redis:alpine
#
# The command above will copy docker.io/library/redis:alpine into the quay repository
# quay.io/dhashimo, instead of the default MAIN_REPO
#
#   make MAIN_REPO=quay.io/dhashimo everything
#
# Similarly, the command above will copy and build/generate everything, but
# into the specified MAIN_REPO.
#
# The last operation is to copy images from MAIN_REPO to another repository,
# which needs to be specified.
#
#   make COPY_REPO=192.168.0.1:5000/internal copy
#
# The command above will copy all images (replicated, generated, reassembled,
# external and other, as listed below) from the MAIN_REPO into the 'internal'
# repository within the registry running at 192.168.0.1:5000.
#
# Individual items can also be specified.  For that, just add '_copy' to
# their main target names:
#
#   make COPY_REPO=192.168.0.1:5000/internal nghttp2_copy
#   make COPY_REPO=192.168.0.1:5000/internal docker.io/library/redis:alpine_copy
#   make COPY_REPO=192.168.0.1:5000/internal docker.io/nginxinc/nginx-unprivileged_copy
#
# This Makefile is intended to be executed manually, with the login of
# individual users, and not by a robot account on a CI.
#
# ATTENTION: Remember that images copied for the first time into quay may
#            be created as 'private' images, and need to be changed into
#            'public' before they can be used by CI and other users.

# --- configuration ---

# tools
SHELL = /bin/sh
PODMAN = podman
SKOPEO = skopeo

# tool options

# The options below create or transform the images into the docker format.
# That is required for their use with Openshift 3.11
FORMAT_OPTIONS = --format docker
TRANSFORM_OPTIONS = --format v2s2
PLATFORM = linux/amd64,linux/arm64,linux/s390x

# Repositories
MAIN_REPO = quay.io/skupper
COPY_REPO = localhost:5000/local

# Set this to copy a specific tag from quay.io/skupper/skupper-tests on
# the copy operation.  If you do so, include the colon - ":1.5"
SKUPPER_TESTS_IMAGE_VERSION =

# This is the list of replicated images that will be copied to $(MAIN_REPO).  If
# the image specifies a tag, make sure to escape the colon with a backslash
# (such as in docker.io/library/mongo\:5.0).  Otherwise, you'll get an error
# like `multiple target patterns.  Stop.`
REPLICATED_IMAGES := \
	docker.io/library/postgres\:9.5 \


# These manifest lists require special handling; they contain 'images' with
# os/arch/variant "unknown" and the following entry in LayersData.Annotations:
#
#    in-toto.io/predicate-type": "https://slsa.dev/provenance/v0.2
#
# That makes a plain skopeo copy --all --format v2s2 to fail with an error
# like the one below:
#
#    copying image 8/14 from manifest list: creating an updated image manifest: Unknown media type during manifest conversion: "application/vnd.in-toto+json"
#
# (see https://github.com/containers/skopeo/issues/1874)
#
# For that reason, we need to first copy the image locally, change its
# manifest, and only then copy it to its destination.
REASSEMBLED_IMAGES := \
	docker.io/library/mongo\:5.0 \
	docker.io/library/postgres \
	docker.io/library/redis \
	docker.io/nginxinc/nginx-unprivileged \
	docker.io/nginxinc/nginx-unprivileged\:stable-alpine \
	docker.io/library/redis\:alpine \
	docker.io/istio/examples-bookinfo-productpage-v1\:1.19.1 \
	docker.io/istio/examples-bookinfo-ratings-v1\:1.19.1 \
	docker.io/istio/examples-bookinfo-details-v1\:1.19.1 \
	docker.io/istio/examples-bookinfo-reviews-v3\:1.19.1

# These are the images that are generated by this Makefile (as opposed to
# created elsewhere and just copied here).  To add a new image, simply create
# Containerfile.IMAGENAME and add the IMAGENAME below.
GENERATED_IMAGES := \
	hey \
	iperf3 \
	nghttp2 \
	wrk

# wrk2 does not support ARM: https://github.com/giltene/wrk2/issues/104

# These are generated by other repositories under https://github.com/skupperproject/,
# and placed directly on quay.  We list them here, so they can be part of the list of images
# to be copied to a private repo
OTHER_IMAGES := \
	quay.io/skupper/wrk2 \
	quay.io/skupper/tcp-go-echo \
	quay.io/skupper/hello-world-frontend \
	quay.io/skupper/hello-world-backend \
	quay.io/skupper/skupper-tests${SKUPPER_TESTS_IMAGE_VERSION}

# These images are external to the project, and they are not copied into our quay;
# the tests access them with their original names, and we do not do any special
# handling on them.  They are listed here just so they can be used on the copy
# operation.
EXTERNAL_IMAGES := \
	quay.io/ssorj/quiver \
	gcr.io/google-samples/microservices-demo/adservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/cartservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/checkoutservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/currencyservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/emailservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/frontend\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/loadgenerator\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/paymentservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/productcatalogservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/recommendationservice\:v0.10.1 \
	gcr.io/google-samples/microservices-demo/shippingservice\:v0.10.1 \
	registry.access.redhat.com/rhscl/postgresql-95-rhel7

# --- end of configuration ---


# These are the build target names, such as nghttp2_build, generated
# automatically from GENERATED_IMAGES.  Do not overwrite this
generated_build := $(patsubst %,%_build,$(GENERATED_IMAGES))

# The copy target names, to be used with the copy operation
generated_copy := $(patsubst %,%_copy,$(GENERATED_IMAGES))
replicated_copy   := $(patsubst %,%_copy,$(REPLICATED_IMAGES))
reassembled_copy := $(patsubst %,%_copy,$(REASSEMBLED_IMAGES))
external_copy := $(patsubst %,%_copy,$(EXTERNAL_IMAGES))
other_copy := $(patsubst %,%_copy,$(OTHER_IMAGES))

# We don't want someone to just run `make` and start copying stuff
# around, so instead the default operation is to just give some
# hint on how to use it.
all:
	@echo "Use 'make everything' for building and copying everything to the MAIN_REPO".
	@echo
	@echo "MAIN_REPO=$(MAIN_REPO)"
	@echo
	@echo Normal use, however, is to select individual targets.
	@echo Check Makefile contents for documentation.

# For a generated image, we simply have a Containerfile named after it and call
# `podman build` on it, tagging it on the MAIN_REPO
# TODO: list Containerfiles as pre-reqs?
$(generated_build): TARGET = $(patsubst %_build,%,$@)
$(generated_build):
	# Generate $(TARGET)
# First, we need to remove any existing manifest, as the command below is additive
# (it would append builds to an existing manifest)
	-$(PODMAN) manifest rm $(MAIN_REPO)/$(TARGET)
	$(PODMAN) build --platform $(PLATFORM) --no-cache $(FORMAT_OPTIONS) --file Containerfile.$(TARGET) --manifest $(MAIN_REPO)/$(TARGET)

# This is the generic push target, for images built here (GENERATED_IMAGES)
# Each image depends on its respective _build target
%: %_build
	$(PODMAN) manifest push $(MAIN_REPO)/$@

# Shortcut targets
replicated_images: $(REPLICATED_IMAGES)
reassembled_images: $(REASSEMBLED_IMAGES)
generated_images: $(GENERATED_IMAGES)
# note OTHER_IMAGES and EXTERNAL_IMAGES are not part of 'everything', as these
# are already in the places where the tests get them from; they're used only
# on the copy operation
everything: replicated_images generated_images reassembled_images

# This is the main target for the replicated images; it copies them from their
# original locations _into_ MAIN_REPO.  Do not confuse this for the `copy` operation
# below, that copies _from_ MAIN_REPO into COPY_REPO.
#
# The first line prepares a target-local TARGET variable, which contains only
# the last part of the URL (the image name proper + tag)
$(REPLICATED_IMAGES): TARGET = $(shell echo "$@" | sed s_.*/__ )
$(REPLICATED_IMAGES):
	# Copy $(TARGET) from the replicated list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$@ \
		docker://$(MAIN_REPO)/$(TARGET)

# This is the main target for the reassembled images; it copies them from their
# original locations to a temporary directory, modifies the manifest.json and then
# copies them _into_ MAIN_REPO.
#
# The first line prepares a target-local TARGET variable, which contains only
# the last part of the URL
$(REASSEMBLED_IMAGES): TARGET = $(shell echo "$@" | sed s_.*/__ )
$(REASSEMBLED_IMAGES): REASSEMBLE_TMPDIR ::= /tmp/$(shell mktemp -d image-reassembly-XXX )
$(REASSEMBLED_IMAGES):
	# Reassemble $(TARGET)
	$(SKOPEO) copy --all \
		docker://$@ \
		dir://$(REASSEMBLE_TMPDIR)

	ls $(REASSEMBLE_TMPDIR)

	jq \
		'(.manifests[] | select (.platform.os == "unknown")) |= del (.) | del (..|nulls)' \
		$(REASSEMBLE_TMPDIR)/manifest.json > $(REASSEMBLE_TMPDIR)/manifest.json.new

	mv $(REASSEMBLE_TMPDIR)/manifest.json.new $(REASSEMBLE_TMPDIR)/manifest.json

	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		dir://$(REASSEMBLE_TMPDIR) \
		docker://$(MAIN_REPO)/$(TARGET)

	rm -rf $(REASSEMBLE_TMPDIR)

#
# Copy operation
#
copy: $(generated_copy) $(replicated_copy) $(reassembled_copy) $(external_copy) $(other_copy)

# The targets for replicated and generated images are basically the same; the
# only difference is the way the original/main target needs to be manipulated
# to generate the image name+tag (saved in the target-local variable TARGET).

$(generated_copy): TARGET = $(patsubst %_copy,%,$@)
$(generated_copy):
	# copy $(TARGET) from generated list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$(MAIN_REPO)/$(TARGET) \
		docker://$(COPY_REPO)/$(TARGET)

# Remove _copy, but also everything till the last "/" (registry and prefix),
# as we'll use MAIN_REPO and COPY_REPO, instead.
$(replicated_copy): TARGET = $(shell echo "$@" | sed -e "s/_copy$$//" -e 's_.*/__' )
$(replicated_copy):
	# copy $(TARGET) from the replicated list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$(MAIN_REPO)/$(TARGET) \
		docker://$(COPY_REPO)/$(TARGET)

$(reassembled_copy): TARGET = $(shell echo "$@" | sed -e "s/_copy$$//" -e 's_.*/__' )
$(reassembled_copy):
	# copy $(TARGET) from the reassembled list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$(MAIN_REPO)/$(TARGET) \
		docker://$(COPY_REPO)/$(TARGET)

# OTHER_IMAGES reside on quay.io/skupper, but are not originally generated or
# copied by this Makefile; still, when doing the copy operation, we can set them
# to be read from a different MAIN_REPO
$(other_copy): TARGET = $(shell echo "$@" | sed -e "s/_copy$$//" -e 's_.*/__' )
$(other_copy):
	# copy $(TARGET) from the other list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$(MAIN_REPO)/$(TARGET) \
		docker://$(COPY_REPO)/$(TARGET)

# Here, the copy is from the original registry
$(external_copy): TARGET = $(patsubst %_copy,%,$@)
$(external_copy):
	# copy $(TARGET) from the external list
	$(SKOPEO) copy --all $(TRANSFORM_OPTIONS) \
		docker://$(TARGET) \
		docker://$(COPY_REPO)/$(TARGET)
