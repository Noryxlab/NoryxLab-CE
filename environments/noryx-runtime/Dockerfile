# The base for an application or a job that has no image of its own.
#
# Until this existed, the reflex was to launch a service on an IDE image. The
# accounting application ran on `noryx-rstudio`: 2.58 GB to host a FastAPI
# service, because that was the smallest thing on the shelf that had Python.
# The cost is not only the pull:
#
#   - a pod that mounts regulated data carried a VS Code server, a VNC stack
#     and a package installer it had no use for;
#   - and the nightly rebuild changed that service's base image whenever a VS
#     Code extension moved upstream, which is an uncontrolled update path for
#     something whose whole virtue is staying put.
#
# So this image carries what the platform's own bootstrap needs and nothing
# else: Python, pip, git, curl, sudo, tini. Measured against the shelf it
# replaces - rstudio 2.58 GB, vscode 1.52 GB, jupyter 497 MB.
#
# What it deliberately does NOT carry is a compiler. An application whose
# dependencies have no wheel needs its own image, built once, like
# `accounting:2026.10.20.5` - 326 MB with everything baked. Adding
# build-essential here would grow every service by 200 MB to serve the
# exception, and would hide the moment where an image of one's own is the right
# answer.
FROM python:3.12-slim-bookworm

# Caches go to the personal volume, never to the ephemeral disk.
#
# This matters more here than anywhere else, because this image is for
# workloads that install their packages at launch. Three paths survive a
# workload: /mnt, /home/noryx/.noryx-profile and /datasets. Everything else,
# the home directory included, is wiped and counted against the pod's
# ephemeral allowance - which is how a few large wheels evicted a session on
# EMSE on 2026-10-20, having also lost the install at the previous restart.
ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    XDG_CACHE_HOME=/home/noryx/.noryx-profile/.cache \
    PIP_CACHE_DIR=/home/noryx/.noryx-profile/.cache/pip \
    UV_CACHE_DIR=/home/noryx/.noryx-profile/.cache/uv \
    HF_HOME=/home/noryx/.noryx-profile/.cache/huggingface \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Where a user's own packages land. pip finds the image's site-packages
# read-only and falls back to a user install; pointing the user base at the
# profile makes that install persist and stop filling the ephemeral disk.
# PIP_USER stays unset on purpose: pip refuses a user install inside a venv,
# and a venv is the normal case for an application.
ENV PYTHONUSERBASE=/home/noryx/.noryx-profile/.local \
    NPM_CONFIG_PREFIX=/home/noryx/.noryx-profile/.local \
    PATH=/home/noryx/.noryx-profile/.local/bin:$PATH

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git sudo tini \
 && rm -rf /var/lib/apt/lists/*

# The same identity every other environment uses. The platform chowns volumes
# to noryx:noryx and runs the pod as that user, so an image that invents its
# own uid gets an unwritable profile and a bootstrap that fails on its first
# mkdir.
RUN useradd -m -s /bin/bash noryx \
 && usermod -aG sudo noryx \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /extracts /home/noryx/.noryx-profile \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /extracts

ENTRYPOINT ["/usr/bin/tini", "--"]

# No application, and no port: the platform supplies the command for an
# application or a job. This default exists only so that launching the image
# bare says why nothing happened, rather than leaving a container that looks
# idle for no stated reason.
CMD ["sh", "-lc", "echo '[noryx-runtime] this image carries no application; the platform supplies the command. Nothing to run.' >&2; exit 64"]

USER noryx
