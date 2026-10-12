#!/usr/bin/env bash
# Pulls Docker Hub official images from mirror.gcr.io and tags each under its
# Docker Hub name, so a later `docker build` or `docker compose build` finds the
# base in the local store and never asks Docker Hub for it. Docker Hub limits
# anonymous pulls per runner address; the mirror does not. An image the mirror
# cannot serve is left for the build to pull from Docker Hub, as it did before.
# Usage: mirror-images.sh golang:1.27-alpine alpine:3.22
set -uo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: mirror-images.sh IMAGE..." >&2
  exit 2
fi

for image in "$@"; do
  mirrored="mirror.gcr.io/library/${image}"
  pulled=false
  for attempt in 1 2 3; do
    if docker pull --quiet "$mirrored" >/dev/null; then
      pulled=true
      break
    fi
    sleep $((attempt * 5))
  done
  if [ "$pulled" = true ]; then
    docker tag "$mirrored" "$image"
    echo "$image <- $mirrored"
  else
    echo "::warning::$mirrored could not be pulled; the build will pull $image from Docker Hub"
  fi
done
