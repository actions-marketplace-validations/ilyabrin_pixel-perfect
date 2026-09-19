FROM golang:1.21-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./

ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /pp .

# The binary is static and needs nothing from the base image.
FROM scratch
COPY --from=build /pp /pp
ENTRYPOINT ["/pp"]
