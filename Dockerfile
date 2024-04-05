FROM golang:1.22-alpine3.19

WORKDIR /app

COPY main.go /app
COPY scele_config.json /app
COPY config.json /app
COPY go.sum /app
COPY go.mod /app
COPY token.json* /app

EXPOSE 9999

CMD [ "go", "run", "." ]
