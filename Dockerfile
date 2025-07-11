FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main .


FROM alpine:latest
COPY --from=build /app/main /main
COPY data/plugin_metadata.json plugin_metadata.json
COPY data/plugin_packs.json plugin_packs.json

EXPOSE 3306

CMD ["./main"]