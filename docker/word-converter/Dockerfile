FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libreoffice-writer python3-uno poppler-utils fonts-liberation fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /work /input

ENV LANG=C.UTF-8 LC_ALL=C.UTF-8 HOME=/work/home TMPDIR=/work/tmp \
    XDG_CACHE_HOME=/work/cache SAL_USE_VCLPLUGIN=svp PYTHONDONTWRITEBYTECODE=1
COPY convert.py /opt/quizzivy-convert.py
USER 65532:65532
WORKDIR /work
ENTRYPOINT ["timeout", "--signal=KILL", "120s", "python3", "/opt/quizzivy-convert.py"]
