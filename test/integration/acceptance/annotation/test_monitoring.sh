KUBECONFIG1=${KUBECONFIG:$1}
KUBECONFIG2=${KUBECONFIG:$2}
kc1="kubectl --kubeconfig=${KUBECONFIG1} -n public-annotated-1 "
kc2="kubectl --kubeconfig=${KUBECONFIG2} -n private-annotated-1 "

annotated_resources_1=(
    "deployment/nginx"
    "service/nginx-1-svc-exp-notarget"
    "service/nginx-1-svc-target"
)

annotated_resources_2=(
    "deployment/nginx"
    "service/nginx-2-svc-exp-notarget"
    "service/nginx-2-svc-target"
)

expected_services=(
    "service/nginx-1-dep-web"
    "service/nginx-1-svc-exp-notarget"
    "service/nginx-1-svc-target"
    "service/nginx-2-dep-web"
    "service/nginx-2-svc-exp-notarget"
    "service/nginx-2-svc-target"
)

function getAnnotatedResources() {
    echo
    echo ">>> Annotated resources from public-annotated-1"
    for res in ${annotated_resources_1[@]}; do
        echo "${res} annotations:"
        $kc1 get ${res} -o template="{{.metadata.annotations}}"
        echo
        echo
    done
    echo
    echo ">>> Annotated resources from private-annotated-1"
    for res in ${annotated_resources_2[@]}; do
        echo "${res} annotations:"
        $kc2 get ${res} -o template="{{.metadata.annotations}}"
        echo
        echo
    done
}

function getServices() {
    echo
    echo ">>> Services and annotations from public-annotated-1"
    for res in ${expected_services[@]}; do
        echo "${res} annotations:"
        $kc1 get ${res} -o template="{{.metadata.annotations}}"
        echo
        echo
    done
    echo
    echo ">>> Services and annotations from private-annotated-1"
    for res in ${expected_services[@]}; do
        echo "${res} annotations:"
        $kc2 get ${res} -o template="{{.metadata.annotations}}"
        echo
        echo
    done
}

#// deployment/nginx  ## to both clusters
#//   annotations:
#//     skupper.io/proxy: tcp
#// service/nginx-1-svc-exp-notarget  ## cluster1
#//   annotations:
#//     skupper.io/proxy: tcp
#// service/nginx-1-svc-target  ## cluster1
#//   annotations:
#//     skupper.io/proxy: http
#//     skupper.io/address: nginx-1-svc-exp-target
#// service/nginx-2-svc-exp-notarget  ## cluster2
#//   annotations:
#//     skupper.io/proxy: tcp
#// service/nginx-2-svc-target  ## cluster2
#//   annotations:
#//     skupper.io/proxy: http
#//     skupper.io/address: nginx-1-svc-exp-target

getAnnotatedResources
getServices

