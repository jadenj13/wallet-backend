FROM golang:1.25-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache libc6-compat
COPY --from=build /bin/api /bin/api
EXPOSE 3000
ENTRYPOINT ["/bin/api"]
