FROM golang:1.22 as builder

WORKDIR /usr/src/app

COPY . .

WORKDIR /usr/src/app/api

RUN go mod tidy && go mod download && go mod verify

RUN CGO_ENABLED=0 GOOS=linux go build -o /behaviour-logger


#####################################################

FROM alpine:3.19.1

COPY --from=builder behaviour-logger behaviour-logger
ENTRYPOINT ["/behaviour-logger"]
