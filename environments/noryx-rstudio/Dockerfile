FROM rocker/rstudio:4.5

# Les caches vont sur le volume personnel, pas dans l'ephemere.
#
# Trois chemins seulement survivent a l'arret d'un workspace : /mnt (le
# projet), /home/noryx/.noryx-profile (la personne) et /datasets. Tout le
# reste, dossier personnel compris, est efface - et decompte du plafond de
# stockage ephemere du pod.
#
# pip, uv et huggingface ecrivent par defaut sous ~/.cache, donc dans
# l'ephemere : un environnement Python un peu fourni remplissait le quota sans
# que rien ne le dise, la session etait evincee, et ce qui avait ete installe
# aurait de toute facon disparu a l'arret suivant. Rencontre sur EMSE le
# 2026-10-20 : deux symptomes pour une seule cause.
#
# Les pointer sur le volume personnel regle les deux : le quota ephemere cesse
# de se remplir, et un paquet telecharge une fois est retrouve a la session
# suivante.
ENV DEBIAN_FRONTEND=noninteractive \
    NORYX_WORKSPACE_IDE=rstudio \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    XDG_CACHE_HOME=/home/noryx/.noryx-profile/.cache \
    PIP_CACHE_DIR=/home/noryx/.noryx-profile/.cache/pip \
    UV_CACHE_DIR=/home/noryx/.noryx-profile/.cache/uv \
    HF_HOME=/home/noryx/.noryx-profile/.cache/huggingface \
    RENV_PATHS_CACHE=/home/noryx/.noryx-profile/.cache/renv

# Where a user's own packages are installed.
#
# pip finds the interpreter's site-packages read-only - it belongs to the image -
# and says so before falling back to a user install: "Defaulting to user
# installation because normal site-packages is not writeable". That fallback
# landed in ~/.local, which is the pod's ephemeral disk: a few large wheels
# (torch and its CUDA dependencies) filled the 8Gi allowance and the session was
# evicted mid-install, having also lost every package at the previous restart.
# Pointing the user base at the profile makes an install persist and stop
# counting against the ephemeral allowance; a virtual environment is unaffected,
# which is why PIP_USER is deliberately not set (pip refuses a user install
# inside a venv).
ENV PYTHONUSERBASE=/home/noryx/.noryx-profile/.local \
    NPM_CONFIG_PREFIX=/home/noryx/.noryx-profile/.local \
    PATH=/home/noryx/.noryx-profile/.local/bin:$PATH \
    R_LIBS_USER=/home/noryx/.noryx-profile/.local/R/%p-library/%v

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git python3 python3-pip python3-venv sudo tini \
 && rm -rf /var/lib/apt/lists/*

RUN usermod -l noryx -d /home/noryx -m rstudio \
 && groupmod -n noryx rstudio \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /extracts /home/noryx/.noryx-profile /var/lib/rstudio-server \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /extracts /var/lib/rstudio-server

EXPOSE 8888
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/usr/lib/rstudio-server/bin/rserver", "--server-daemonize=0", "--www-address=0.0.0.0", "--www-port=8888", "--auth-none=1", "--server-user=noryx", "--server-working-dir=/mnt"]
