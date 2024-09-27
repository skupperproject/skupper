QUAY_ORG ?= ckruse

IMAGE_NAME ?= quay.io/${QUAY_ORG}/skupper-kiali-bridge:latest

build:
	docker build . -t ${IMAGE_NAME}

push:
	docker push ${IMAGE_NAME}

.ensure-namespace:
	@if [ -z "${NAMESPACE}" ]; then echo "NAMESPACE env var is not set. Aborting."; exit 1; fi

deploy: .ensure-namespace
	@cat ./deployment.yaml | IMAGE_NAME="${IMAGE_NAME}" NAMESPACE=${NAMESPACE} envsubst | kubectl apply -n ${NAMESPACE} -f -

undeploy: .ensure-namespace
	@cat ./deployment.yaml | IMAGE_NAME="${IMAGE_NAME}" NAMESPACE=${NAMESPACE} envsubst | kubectl delete -n ${NAMESPACE} -f -

go-build:
	go build
