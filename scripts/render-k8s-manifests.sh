#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <overlay-path> <api-gateway-image> <message-engine-image>" >&2
  exit 1
fi

overlay_path="$1"
api_gateway_image="$2"
message_engine_image="$3"

kubectl kustomize "$overlay_path" | sed \
  -e "s|image: agentmsg/api-gateway:[^[:space:]]*|image: ${api_gateway_image}|g" \
  -e "s|image: agentmsg/message-engine:[^[:space:]]*|image: ${message_engine_image}|g"
