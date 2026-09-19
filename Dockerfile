# Always build on the runner's native architecture and cross-compile from
# there: Go does this in seconds, where emulating the target under QEMU takes
# minutes.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=docker

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /pp .

# The binary is static and needs nothing from the base image.
FROM scratch
COPY --from=build /pp /pp
ENTRYPOINT ["/pp"]
