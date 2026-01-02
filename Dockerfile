FROM golang:1.22 AS builder

WORKDIR $GOPATH/src/webserver
COPY . ./

RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash -s -- -v 0.13.4 -p /usr/local
RUN bash -c "ls -l /usr/local/lib"
RUN go get github.com/second-state/WasmEdge-go/wasmedge@v0.13.4
RUN go get github.com/second-state/wasmedge-bindgen@v0.4.1

RUN GOOS=linux go build -a -installsuffix nocgo -o /webserver cmd/main.go
COPY functions/benchmarks-v2/*.wasm /

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y lsof htop procps
RUN apt-get update && apt-get install -y curl git python3

COPY --from=builder /webserver  ./
RUN mkdir -p /functions
COPY --from=builder /*.wasm  ./functions/
RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash -s -- -v 0.13.4 -p /usr/local

CMD ["./webserver"]
EXPOSE 8095