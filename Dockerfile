# syntax=docker/dockerfile:1.6
#####################
### Server Build Step
#####################
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.22.4-alpine3.19 AS server-builder 

# Install build dependencies
RUN apk add --no-cache make gcc musl-dev git bash curl ca-certificates tzdata

WORKDIR /server

# Copy only files needed for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy only what's needed for the server build
COPY Makefile ./
COPY main.go ./
COPY app/ ./app/
COPY migrations/ ./migrations/
COPY views/ ./views/
COPY locale/ ./locale/
COPY misc/ ./misc/
COPY etc/ ./etc/

# Build the application
ARG COMMITHASH=""
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV COMMITHASH=${COMMITHASH}
# Print Go information for debugging
RUN go version && \
    go env && \
    echo "Building fider..." && \
    go build -v -ldflags="-s -w" -o fider . || \
    (echo "Go build failed!" && ls -la && exit 1)

#################
### UI Build Step
#################
FROM --platform=${TARGETPLATFORM:-linux/amd64} node:22.5-alpine AS ui-builder 

WORKDIR /ui

# Copy only files needed for dependencies
COPY package.json package-lock.json tsconfig.json .nvmrc ./
# Install make for the build commands
RUN apk add --no-cache make
RUN npm ci --no-audit --prefer-offline --no-fund

# Copy only what's needed for UI build
COPY public/ ./public/
COPY webpack.config.js ./
COPY esbuild.config.js ./
COPY esbuild-shim.js ./
COPY lingui.config.js ./
COPY .babelrc ./
COPY Makefile ./
COPY favicon.png ./
COPY robots.txt ./
COPY robots-dev.txt ./

# Build with production settings
ENV NODE_ENV=production
RUN make build-ssr build-ui

################
### Runtime Step
################
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.19

# Install only required runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy application files with consistent ordering to improve cache efficiency
COPY --from=server-builder /server/fider /app/
COPY --from=server-builder /server/migrations/ /app/migrations/
COPY --from=server-builder /server/views/ /app/views/
COPY --from=server-builder /server/locale/ /app/locale/
COPY --from=server-builder /server/misc/ /app/misc/
COPY --from=server-builder /server/etc/ /app/etc/
COPY --from=server-builder --chown=appuser:appuser /server/LICENSE /app/

# Copy UI files with consistent ordering
COPY --from=ui-builder /ui/favicon.png /app/
COPY --from=ui-builder /ui/dist/ /app/dist/
COPY --from=ui-builder /ui/robots-dev.txt /app/
COPY --from=ui-builder /ui/robots.txt /app/
COPY --from=ui-builder /ui/ssr.js /app/

# Set proper ownership and permissions
RUN chown -R appuser:appuser /app && \
    chmod +x /app/fider

# Switch to non-root user
USER appuser

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD /app/fider ping

# Use array syntax for better signal handling
ENTRYPOINT ["/app/fider"]
CMD ["migrate", "&&", "/app/fider"]