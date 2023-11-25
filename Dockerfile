FROM golang:1.21 

WORKDIR /usr/src/app

COPY . .

WORKDIR /usr/src/app/api

RUN go mod tidy && go mod download && go mod verify
RUN go build -v -o /usr/local/bin/app ./...

CMD ["app"]

