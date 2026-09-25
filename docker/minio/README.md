# Local S3 test service

The local and CI MinIO image is built from official source commits. The former
Quay and Docker Hub images refuse anonymous pulls, and the historical binary
download endpoints return HTTP 410. No third-party binary mirror is used.

| Binary | Existing release | Source commit |
| --- | --- | --- |
| MinIO | `RELEASE.2025-09-07T16-13-09Z` | [07c3a429bfed433e49018cb0f78a52145d4bedeb](https://github.com/minio/minio/commit/07c3a429bfed433e49018cb0f78a52145d4bedeb) |
| mc | `RELEASE.2025-08-13T08-35-41Z` | [7394ce0dd2a80935aded936b09fa12cbb3cb8096](https://github.com/minio/mc/commit/7394ce0dd2a80935aded936b09fa12cbb3cb8096) |

`docker compose build minio` builds both binaries sequentially with two Go
threads, one compiler process, and a 512 MiB Go memory target. BuildKit caches
modules and compilation on Docker's disk. The first build needs network access
and takes longer than pulling the former image; subsequent builds reuse it.
`make up` builds the image before starting the stack. Both services share the image,
and `scripts/verify-r2.sh` uses its client too.

The image contains the upstream license and credits for each binary. It is a
development/CI stand-in for production Cloudflare R2; these historical releases
are not a supported production storage deployment. Changing this image does not
remove the local `minio-data` volume.
