FROM node:24.11.1 AS build-deps

WORKDIR /app/web
COPY ./package.json .
COPY ./package-lock.json .
RUN npm ci


FROM build-deps AS build

WORKDIR /app/web
COPY . .

# explicitly choose .env.prod
COPY .env.prod .env
ARG NEXT_PUBLIC_DEFAULT_RESOLVER=127.0.0.11:53
RUN npm run build

FROM docker.io/library/caddy:latest AS prod-caddy

COPY --from=build /app/web/out /usr/share/nginx/html
COPY --from=build /app/web/.version.txt /usr/share/nginx/html/version.txt


FROM docker.io/library/nginx:alpine AS prod-nginx

COPY --from=build /app/web/out /usr/share/nginx/html
COPY --from=build /app/web/.version.txt /usr/share/nginx/html/version.txt
