FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go test ./... && go build -o /out/scheduler-simulator .

FROM alpine:3.21

WORKDIR /app
COPY --from=build /out/scheduler-simulator /app/scheduler-simulator

EXPOSE 8000
CMD ["/app/scheduler-simulator"]
