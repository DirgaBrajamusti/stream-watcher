FROM node:16 as web
WORKDIR /helpers/webserver/frontend
COPY /helpers/webserver/frontend .
RUN yarn install
RUN yarn build

FROM golang:1.20-alpine AS ytarchive-builder
WORKDIR /src
RUN set -ex; \
    apk add --no-cache git; \
    git clone https://github.com/Kethsar/ytarchive.git; \
    cd ytarchive; \
    go build .

FROM golang:1.23-alpine as build
WORKDIR /app
COPY . .
COPY --from=web /helpers/webserver/frontend/dist /app/helpers/webserver/frontend/dist
RUN go build -o super-bad-stream-watcher ./cmd

FROM alpine as runner
WORKDIR /app
RUN apk update && apk add --no-cache ffmpeg curl bash streamlink
RUN curl https://i.jpillora.com/yt-dlp/yt-dlp! | bash
COPY --from=ytarchive-builder /src/ytarchive/ytarchive /usr/local/bin/ytarchive
COPY --from=build /app/super-bad-stream-watcher /app
CMD ["/app/super-bad-stream-watcher"]
