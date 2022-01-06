FROM golang:1.17-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /vcse

FROM golang:1.17-alpine

WORKDIR /

COPY --from=build /vcse /vcse

EXPOSE 8090

ENTRYPOINT ["/vcse"]
