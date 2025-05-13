#! /bin/bash
# Use these environment variables:
# OCP_TOKEN=xxx
# OCP_SERVER_URL=https://api.migrationlab.devopstribe.it:6443
# OCP_NAMESPACES=migrationlab,default

if [[ -z "$OCP_TOKEN" || -z "$OCP_SERVER_URL" || -z "$OCP_NAMESPACES" ]]; then
  echo "Error: One or more required environment variables (OCP_TOKEN, OCP_SERVER_URL, OCP_NAMESPACES) are not set or empty."
  exit 1
fi

go run main.go start --namespaces $OCP_NAMESPACES --token $OCP_TOKEN --server-url $OCP_SERVER_URL 


