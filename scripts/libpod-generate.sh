# Based on https://storage.googleapis.com/libpod-master-releases/swagger-v4.0.3.yaml
# A few changes have been done to the original spec due to some issues, like:
# https://github.com/containers/podman/issues/13092
#
# * IdResponse type (removed as not used and causing inconsistencies with IDResponse)
# * LibpodImageSummary (conflicts with ImageSummary)
#   - Removed x-go-name: ImageSummary
# * Mount type
#   - Added Destination (string)
#   - Added Options ([]string)
# 
LIBPOD_SPEC='./scripts/swagger-v4.0.3.yaml'

FORCE="${FORCE:-false}"

[ -d client/generated/libpod ] && ! ${FORCE} && exit 0

# Generating libpod clients
./scripts/swagger-generate.sh client/generated/libpod ${LIBPOD_SPEC}
