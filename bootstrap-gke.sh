#!/bin/bash

# Requires: jq, vault, correctly configured kubectl and all the correct rights
# Usage example: ./bootstrap.sh ns stg ns-gke-stg-usc1 your_idm_user kubectl_context
# For usage on a mac, requires gnu-sed which can be installed via brew install gnu-sed

set -e
set -u

export VAULT_ADDR="https://vault.corp"
namespace=$1
environment=$2
cluster=$3
username=$4
context=$5

function add_resource {
  namespace=$1
  name=$2
  type=$3
  yaml=$4

  if [ "$(kubectl get ${type} -n ${namespace} ${name} | grep ${name})" ]; then
    kubectl replace -n ${namespace} -f $yaml
  else
    kubectl apply -n ${namespace} -f $yaml
  fi
}

kubectl config use-context $context
kubernetes_host=$(kubectl cluster-info | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | sed -E -n 's#.*Kubernetes master.* is running at .*(https://.*$).*#\1#p')
kubectl create serviceaccount -n "${namespace}" ${namespace}-vault-sa || true

kubernetes_cluster=$(mktemp)
sed "s#NAMESPACE#${namespace}#g" manifests-gke/cluster.yaml | sed "s#CLUSTER#${cluster}#g" >${kubernetes_cluster}
add_resource ${namespace} kubernetes-cluster configmap ${kubernetes_cluster}
rm ${kubernetes_cluster}

rbac_conf=$(mktemp)
sed "s#NAMESPACE#${namespace}#g" manifests-gke/rbac_conf.yaml >${rbac_conf}
add_resource ${namespace} "${namespace}-role-tokenreview-binding" clusterrolebinding ${rbac_conf}
rm ${rbac_conf}

sa_token=$(mktemp)
sed "s#NAMESPACE#${namespace}#g" manifests-gke/sa_token.yaml >${sa_token}
add_resource ${namespace} "${namespace}-vault-sa-secret" secret ${sa_token}
rm ${sa_token}

jwt_token=$(kubectl get secrets -n ${namespace} ${namespace}-vault-sa-secret -o json | jq -r .data.token | base64 --decode)
ca_crt=$(kubectl get secrets -n ${namespace} ${namespace}-vault-sa-secret -o json | jq -r '.data["ca.crt"]' | base64 --decode)

vault login -method=okta username=${username}
vault auth enable -path=kubernetes-${cluster}/ kubernetes || true

vault_policy=$(echo 'path "SECRET_PATH/*" { capabilities = ["read"] }' | sed "s#SECRET_PATH#secret/${environment}/${namespace}#")
echo "${vault_policy}" | vault policy write ${cluster}-${namespace}-vault-sa-${environment} - || true

vault write auth/kubernetes-${cluster}/config token_reviewer_jwt="${jwt_token}" kubernetes_host="${kubernetes_host}" kubernetes_ca_cert="${ca_crt}"
vault write auth/kubernetes-${cluster}/role/${namespace}-vault-sa bound_service_account_names=${namespace}-vault-sa bound_service_account_namespaces="${namespace}" policies="${cluster}-${namespace}-vault-sa-${environment}" ttl=1h

test_result=$(curl -sL -o /dev/null -w "%{http_code}" -d "{ \"jwt\": \"${jwt_token}\", \"role\": \"${namespace}-vault-sa\" }" "${VAULT_ADDR}/v1/auth/kubernetes-${cluster}/login")
if [ "${test_result}" == "200" ]; then
  echo "Bootstrapping finished properly"
else
  echo "Something failed in the bootstrapping, can't authenticate to vault with the JWT token and role ${namespace}-vault-sa"
fi
