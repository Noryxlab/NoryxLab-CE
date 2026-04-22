FROM debian:12-slim

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    ca-certificates \
    python3 \
    python3-pip \
    tini \
 && rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --break-system-packages --no-cache-dir \
    jupyterlab==4.2.5 \
    notebook==7.2.2

EXPOSE 8888

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["python3", "-m", "jupyterlab", "--ip=0.0.0.0", "--port=8888", "--no-browser"]
