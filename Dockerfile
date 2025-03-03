FROM golang:1.23.3 AS build
ARG SERVER_VERSION
ARG DATETIME

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY api/ ./api/
COPY cmd/ ./cmd/
COPY impl/ ./impl/

# make a dir we know is empty
RUN mkdir /var/lib/emptydir

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build\
    -ldflags "-X 'main.buildVer=v$SERVER_VERSION' -X 'main.buildDtm=$DATETIME'" -a -o server cmd/*.go

FROM gcr.io/distroless/static:nonroot

WORKDIR /ociregistry
COPY --from=build /app/server .
# copy emptydir with correct permissions for mounted volume to inherit
COPY --from=build --chown=nonroot:nonroot /var/lib/emptydir /var/lib/ociregistry
USER nonroot:nonroot

ENTRYPOINT ["/ociregistry/server"]
