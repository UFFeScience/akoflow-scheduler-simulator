FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go test ./... && go build -o /out/scheduler-simulator .

FROM golang:1.23-alpine AS debug

ENV PATH="/usr/local/go/bin:/go/bin:${PATH}"

WORKDIR /app

RUN go install github.com/go-delve/delve/cmd/dlv@v1.25.0
RUN ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt \
    && ln -sf /go/bin/dlv /usr/local/bin/dlv

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

EXPOSE 8000 2345
CMD ["dlv", "dap", "--listen=:2345", "--log", "--log-output=dap"]

FROM alpine:3.21

WORKDIR /app
COPY --from=build /out/scheduler-simulator /app/scheduler-simulator

EXPOSE 8000
CMD ["/app/scheduler-simulator"]
