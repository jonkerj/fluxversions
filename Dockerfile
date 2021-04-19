FROM golang:1.15 as builder
RUN mkdir /workdir
WORKDIR /workdir
COPY go.mod deps ./

RUN go mod download
RUN CGO_ENABLED=0 go build $(cat deps)
COPY main.go .
RUN CGO_ENABLED=0 go build -o /app main.go

FROM scratch
# Define GOTRACEBACK to mark this container as using the Go language runtime
# for `skaffold debug` (https://skaffold.dev/docs/workflows/debug/).
ENV GOTRACEBACK=single
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
