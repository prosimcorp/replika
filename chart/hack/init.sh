#!/bin/bash

KUBERNETES_VESION="v1.22.3"
HELM_RELEASE_NAME=test-replika
CHART=../
NAMESPACE=replika
VALUES=../values.yaml

# check minikube
which minikube > /dev/null
if [ "$?" -ne 0 ]; then
  echo "First install minikube. Ref: https://minikube.sigs.k8s.io/docs/start/"
  exit 1
fi

if [ "$1" = "start-test" ]; then
  # start minikube
  minikube start --kubernetes-version=${KUBERNETES_VESION}

  # deploy chart
  helm upgrade -i --debug         \
    ${HELM_RELEASE_NAME} ${CHART} \
    -n ${NAMESPACE}               \
    -f ${VALUES}

  # deploy sample
  kubectl -n ${NAMESPACE} apply -f sample.yaml
fi

if [ "$1" = "stop-test" ]; then
  # clean sample
  kubectl -n ${NAMESPACE} delete -f sample.yaml

  # clean CRD
  kubectl delete crd replikas.replika.prosimcorp.com

  # uninstall chart
  helm uninstall --debug \
    ${HELM_RELEASE_NAME} \
    -n ${NAMESPACE}

  # stop minikube
  minikube stop
fi

if [ "$1" = "upgrade-install" ]; then
  helm upgrade -i --debug         \
    ${HELM_RELEASE_NAME} ${CHART} \
    --create-namespace            \
    -n ${NAMESPACE}               \
    -f ${VALUES}

  # deploy sample
  kubectl -n ${NAMESPACE} apply -f sample.yaml
fi

if [ "$1" = "uninstall" ]; then
  # clean sample
  kubectl -n ${NAMESPACE} delete -f sample.yaml

  # clean CRD
  kubectl delete crd replikas.replika.prosimcorp.com

  # uninstall chart
  helm uninstall --debug \
    ${HELM_RELEASE_NAME} \
    -n ${NAMESPACE}
fi

if [ -z "$1" ]; then
echo """
  Usage:
    init.sh <COMMAND>

  Command:
    start-test       deploy minikube with replika chart
    stop-test        stop minikube and clean replika chart
    uninstall        delete replika release
    upgrade-install  upgrade and install replika chart
  """
fi
