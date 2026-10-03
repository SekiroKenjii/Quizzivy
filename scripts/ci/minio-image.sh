#!/usr/bin/env bash
# Prints the registry reference of the MinIO image built from docker/minio as
# it is at HEAD. The tag is a hash of that directory, so a change to the build
# names a new image and an unchanged build finds the published one.
set -euo pipefail

owner="${GITHUB_REPOSITORY_OWNER:?GITHUB_REPOSITORY_OWNER is not set}"
tag="$(git ls-tree -r HEAD -- docker/minio | sha256sum | cut -c1-20)"
echo "ghcr.io/${owner,,}/quizzivy-minio:${tag}"
