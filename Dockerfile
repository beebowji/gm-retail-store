FROM golang:1.21.0-alpine AS build_base

ARG ORG_NAME
ARG GIT_HOST
ARG GIT_AUTH_USER
ARG GIT_AUTH_PASS

RUN apk add alpine-sdk
RUN apk --update add build-base
RUN apk --update add git


# Set the Current Working Directory inside the container
WORKDIR /app

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go env -w GOPRIVATE=${GIT_HOST}/${ORG_NAME}/*

RUN git config \
    --global \
    url."https://${GIT_AUTH_USER}:${GIT_AUTH_PASS}@${GIT_HOST}".insteadOf \
    "https://${GIT_HOST}"

RUN go mod download

COPY . .

# Build the Go app
# RUN go build -o ./out .
# https://github.com/confluentinc/confluent-kafka-go/issues/461#issuecomment-617591791
RUN GOOS=linux GOARCH=amd64 go build -tags musl -o ./out .


# Start fresh from a smaller image
FROM alpine:3.15.0
RUN apk add ca-certificates

COPY --from=build_base /app/out /app

# This container exposes port 8080 to the outside world
EXPOSE 4000 443

# Run the binary program produced by `go install`
CMD ["/app"]
