FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api

FROM alpine:3.21
COPY --from=build /bin/api /bin/api
EXPOSE 3000
ENTRYPOINT ["/bin/api"]
