set -o errexit
set -o nounset

# Target is a local directory that will be created
TARGET=${1}
# Source can be a local or remote (http) openapi definition
SOURCE=${2}

# Validate if running inside a go module
[ ! -f go.mod ] && echo "Run it from Skupper's repository root, where go.mod is located" && exit 1

# Creating the target location
[ ! -d ${TARGET} ] && mkdir -p ${TARGET}

# Generating the client code
if [ ! -f ./swagger ]; then
    if [ -d ${REMOTE_SOURCES_DIR:-}/swagger/app ]; then
        echo "Building swagger"
        WORKDIR=`pwd`
        ( cd ${REMOTE_SOURCES_DIR:-}/swagger/app && go build -o ${WORKDIR}/swagger ./cmd/swagger/ )
    else
        download_url="https://github.com/go-swagger/go-swagger/releases/download/v0.30.2/swagger_$(go env GOOS)_$(go env GOARCH)"
        curl -o ./swagger -L'#' "$download_url"
        chmod +x ./swagger
    fi
fi

./swagger generate client --keep-spec-order -t "${TARGET}" -f "${SOURCE}"
