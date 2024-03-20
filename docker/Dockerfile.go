ARG GO_VERSION=1.22.0

# Build stage
FROM golang:${GO_VERSION}-bookworm AS run
ARG PROJECT
ARG USER_ID
ARG GROUP_ID

# Determine the group name for the provided GROUP_ID
# If the group doesn't exist, use 'go' as the group name
# This is required because MacOS has strage behavior with GIDs (Default user has GID ~20)
RUN if ! getent group ${GROUP_ID} > /dev/null 2>&1; then \
        GROUP_NAME=go; \
        addgroup -g ${GROUP_ID} ${GROUP_NAME}; \
        echo $GROUP_NAME > /tmp/groupfile; \
    else \
        GROUP_NAME=$(getent group ${GROUP_ID} | cut -d: -f1); \
        echo $GROUP_NAME > /tmp/groupfile; \
    fi
    
# Add the 'go' user with the specified USER_ID and add to the determined group
RUN groupadd -g ${GROUP_ID} go
RUN useradd -u ${USER_ID} -g ${GROUP_ID} -m go
USER go:go

WORKDIR /src
COPY ${PROJECT}/go.mod ${PROJECT}/go.sum ./
RUN go mod download
COPY ${PROJECT}/pkg ./pkg
COPY ${PROJECT}/main.go ./main.go

# Dev stage, can be used for development using docker-compose "build.target" config
FROM golang:${GO_VERSION}-bookworm as we
RUN apt update  && apt install -y xz-utils
ADD https://github.com/watchexec/watchexec/releases/download/v1.25.1/watchexec-1.25.1-x86_64-unknown-linux-gnu.tar.xz .
RUN tar -xvf watchexec-1.25.1-x86_64-unknown-linux-gnu.tar.xz
RUN mv watchexec-1.25.1-x86_64-unknown-linux-gnu/watchexec /usr/local/bin

FROM run as dev
WORKDIR /tmp
USER root
COPY --from=we /usr/local/bin/watchexec /usr/local/bin/watchexec
USER go
COPY ${PROJECT}/docker/dev-entrypoint.sh /dev-entrypoint.sh
WORKDIR /src
RUN mkdir -p /home/go/.cache/go-build
ARG VERSION=dev
ENV VERSION=${VERSION}
ENV HOME=/home/go
ENTRYPOINT ["/dev-entrypoint.sh"]

FROM run as build
WORKDIR /src
RUN CGO_ENABLED=1 go build \
    -installsuffix 'static' \
    -o /home/go/app ./main.go

# Prod stage, uses distrolless image
FROM gcr.io/distroless/base AS prod
ARG VERSION=dev
ENV VERSION=${VERSION}
ENV GIN_MODE=release
USER nonroot:nonroot
COPY --from=build --chown=nonroot:nonroot /home/go/app /home/nonroot/app
LABEL maintainer="https://github.com/glothriel"
ENTRYPOINT ["/home/nonroot/app"]
CMD ["start"]