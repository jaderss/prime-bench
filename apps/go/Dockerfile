ARG GO_VERSION=1.27
FROM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.mod ./
COPY . ./
RUN go vet ./... && go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/prime-bench .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/prime-bench /prime-bench
ENV RESULTS_DIR=/results
EXPOSE 8080
ENTRYPOINT ["/prime-bench"]
