#!/bin/bash

kind delete cluster --name default
kind create cluster --name default

echo "***"
echo "Creating namespace..."
kubectl create ns kubemart-system

echo "***"
echo "Applying RBAC for kubemart-daemon..."
kubectl apply -f https://raw.githubusercontent.com/kubemart/kubemart-daemon/master/rbac.yaml

echo "***"
echo "Counting current installed apps..."
NUM_OF_APPS=$(kubectl -n kubemart-system get apps --no-headers | wc -l | xargs)

if [ $NUM_OF_APPS -ne 0 ]; then
    echo "***"
    echo "Found $NUM_OF_APPS apps - deleting them now..."
    kubectl delete apps --all -A  
fi

echo "***"
echo "Running tests..."
ginkgo --slowSpecThreshold 300 --v ./controllers
