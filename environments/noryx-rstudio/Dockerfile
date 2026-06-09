FROM rocker/rstudio:4.5

ENV DEBIAN_FRONTEND=noninteractive \
    NORYX_WORKSPACE_IDE=rstudio \
    PIP_DISABLE_PIP_VERSION_CHECK=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git python3 python3-pip python3-venv sudo tini \
 && rm -rf /var/lib/apt/lists/*

RUN usermod -l noryx -d /home/noryx -m rstudio \
 && groupmod -n noryx rstudio \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /home/noryx/.noryx-profile /var/lib/rstudio-server \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /var/lib/rstudio-server

EXPOSE 8888
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/usr/lib/rstudio-server/bin/rserver", "--server-daemonize=0", "--www-address=0.0.0.0", "--www-port=8888", "--auth-none=1", "--server-user=noryx", "--server-working-dir=/mnt"]
