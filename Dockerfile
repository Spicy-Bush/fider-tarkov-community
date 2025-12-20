#####################
### UI Build Step (Must run first for embedding)
#####################
FROM --platform=${TARGETPLATFORM:-linux/amd64} node:22-bookworm AS ui-builder 

WORKDIR /ui

COPY package.json package-lock.json ./
RUN npm ci --maxsockets 1

COPY . .
RUN make build-ssr
RUN make build-ui

#####################
### Server Build Step (Embeds all frontend assets)
#####################
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.25.5-bookworm AS server-builder 

RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    libc6-dev

RUN mkdir /server
WORKDIR /server

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

COPY --from=ui-builder /ui/dist /server/dist
COPY --from=ui-builder /ui/ssr.js /server/ssr.js

ARG COMMITHASH
RUN COMMITHASH=${COMMITHASH} GOOS=${TARGETOS} GOARCH=${TARGETARCH} make build-server

################
### Runtime Step (Single binary deployment)
################
FROM --platform=${TARGETPLATFORM:-linux/amd64} debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /app

COPY --from=server-builder /server/fider /app/fider
COPY --from=server-builder /server/LICENSE /app/LICENSE

# Copy origin certs for HTTP/2
COPY etc/ /app/etc/

EXPOSE 3000

HEALTHCHECK --timeout=5s CMD ./fider ping

CMD ./fider migrate && ./fider
