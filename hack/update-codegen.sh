SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# Grab code-generator version from go.sum
CODEGEN_VERSION=$(grep 'k8s.io/code-generator' go.mod | awk '{print $2}')
CODEGEN_PKG=$(echo $(go env GOPATH)"/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}")

if [[ ! -d ${CODEGEN_PKG} ]]; then
    echo "${CODEGEN_PKG} is missing. Running 'go mod download'."
    go mod download
fi

echo ">> Using ${CODEGEN_PKG}"

# Grab openapi-gen version from go.mod
OPENAPI_VERSION=$(grep 'k8s.io/kube-openapi' go.mod | awk '{print $2}')
OPENAPI_PKG=$(echo $(go env GOPATH)"/pkg/mod/k8s.io/kube-openapi@${OPENAPI_VERSION}")

if [[ ! -d ${OPENAPI_PKG} ]]; then
    echo "${OPENAPI_PKG} is missing. Running 'go mod download'."
    go mod download
fi

echo ">> Using ${OPENAPI_PKG}"

# code-generator does work with go.mod but makes assumptions about
# the project living in `$GOPATH/src`. To work around this and support
# any location; create a temporary directory, use this as an output
# base, and copy everything back once generated.
TEMP_DIR=$(mktemp -d)
cleanup() {
    echo ">> Removing ${TEMP_DIR}"
    rm -rf ${TEMP_DIR}
}
trap "cleanup" EXIT SIGINT

echo ">> Temporary output directory ${TEMP_DIR}"

# Ensure we can execute.
chmod +x ${CODEGEN_PKG}/generate-groups.sh

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
cd ${SCRIPT_ROOT}
${CODEGEN_PKG}/generate-groups.sh "all" \
    ms-operator/pkg/client ms-operator/pkg/apis \
    v1 \
    --output-base "${TEMP_DIR}" \
    --go-header-file hack/boilerplate.go.txt


# Notice: The code in code-generator does not generate defaulter by default.
# We need to build binary from vendor cmd folder.
# echo "Building defaulter-gen"
# go build -o defaulter-gen ${CODEGEN_PKG}/cmd/defaulter-gen

# ${GOPATH}/bin/defaulter-gen is automatically built from ${CODEGEN_PKG}/generate-groups.sh
echo "Generating defaulters for mindspore v1"
${GOPATH}/bin/defaulter-gen --input-dirs ms-operator/pkg/apis/v1 \
    -O zz_generated.defaults \
    --output-package ms-operator/pkg/apis/v1 \
    --go-header-file hack/boilerplate.go.txt "$@"
