FROM golang:1.19-alpine3.17 as builder

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) go build -o /k8s-node-check

FROM scratch
COPY --from=builder /k8s-node-check /
ENTRYPOINT [ "/k8s-node-check" ]
