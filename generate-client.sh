# Note:
# As of 16 Feb 2021, this script isn't working properly and we need fix this
# so we can have a Go client -- which can be used in the Bizaar CLI project.
# Otherwise, Bizaar CLI needs to use REST API to CRUD the App CR.
# ---
# https://github.com/slok/kube-code-generator#kubernetes-type-code-generation
# https://github.com/operator-framework/operator-sdk/issues/1975#issuecomment-606903277

export DIRECTORY=$(PWD)
export PROJECT_PACKAGE=github.com/kubemart/kubemart-operator
export GROUPS_VERSION=":v1alpha1"

docker run -it --rm \
    -v $DIRECTORY:/go/src/$PROJECT_PACKAGE \
    -e PROJECT_PACKAGE=$PROJECT_PACKAGE \
    -e CLIENT_GENERATOR_OUT=$PROJECT_PACKAGE/pkg/client \
    -e APIS_ROOT=$PROJECT_PACKAGE/api \
    -e GROUPS_VERSION=$GROUPS_VERSION \
    -e GENERATION_TARGETS="deepcopy,client" \
    quay.io/slok/kube-code-generator:v1.20.1
