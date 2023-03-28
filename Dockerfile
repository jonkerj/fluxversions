FROM --platform=$BUILDPLATFORM golang:1.19 as builder
RUN mkdir /workdir
WORKDIR /workdir
COPY . /workdir/
ARG TARGETOS TARGETARCH

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -o /app main.go

FROM scratch
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
