# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS build

WORKDIR /src/apps/web

COPY apps/web/package.json apps/web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
	npm ci --prefer-offline --no-audit \
		--fetch-retries=5 \
		--fetch-retry-mintimeout=10000 \
		--fetch-retry-maxtimeout=60000

COPY apps/web ./
COPY api/openapi.yaml /src/api/openapi.yaml

RUN npm run typecheck
RUN npm run test
RUN npm run build

FROM caddy:2-alpine AS runtime

RUN cp /usr/bin/caddy /usr/bin/caddy-unprivileged \
	&& mv /usr/bin/caddy-unprivileged /usr/bin/caddy \
	&& addgroup -g 10001 app \
	&& adduser -D -H -u 10001 -G app app

COPY apps/web/Caddyfile /etc/caddy/Caddyfile
COPY --from=build --chown=10001:10001 /src/apps/web/dist /srv

USER 10001:10001
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=6 CMD wget -q --spider http://127.0.0.1:8080/healthz || exit 1

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
