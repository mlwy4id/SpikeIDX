# Worker Python SpikeIDX: supercronic menjalankan `spikeidx worker` tiap 3 jam (WIB).
FROM ghcr.io/astral-sh/uv:0.12.3 AS uv

FROM python:3.13-slim
ARG SUPERCRONIC_VERSION=v0.2.33
ADD https://github.com/aptible/supercronic/releases/download/${SUPERCRONIC_VERSION}/supercronic-linux-amd64 /usr/local/bin/supercronic
RUN chmod +x /usr/local/bin/supercronic
COPY --from=uv /uv /usr/local/bin/uv

ENV TZ=Asia/Jakarta \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    PATH=/app/.venv/bin:$PATH
WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY src ./src
COPY internal/infra/postgres/migrations ./internal/infra/postgres/migrations
RUN uv sync --frozen --no-dev
COPY docker/crontab ./crontab

CMD ["supercronic", "/app/crontab"]
