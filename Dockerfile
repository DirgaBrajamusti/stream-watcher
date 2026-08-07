FROM node:20-alpine AS web
WORKDIR /helpers/webserver/frontend
COPY /helpers/webserver/frontend .
RUN yarn install
RUN yarn build

FROM golang:1.22-alpine AS ytarchive-builder
WORKDIR /src
RUN set -ex; \
    apk add --no-cache git; \
    git clone https://github.com/Kethsar/ytarchive.git; \
    cd ytarchive; \
    go build .

FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
COPY --from=web /helpers/webserver/frontend/dist /app/helpers/webserver/frontend/dist
RUN go build -o super-bad-stream-watcher ./cmd

FROM alpine AS runner
WORKDIR /app
RUN apk add --no-cache ffmpeg curl bash streamlink python3 py3-pip
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && chmod a+rx /usr/local/bin/yt-dlp
COPY --from=ytarchive-builder /src/ytarchive/ytarchive /usr/local/bin/ytarchive
COPY --from=build /app/super-bad-stream-watcher /app/
CMD ["/app/super-bad-stream-watcher"]
