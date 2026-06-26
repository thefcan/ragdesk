# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ragdesk-api .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ragdesk-api /ragdesk-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/ragdesk-api"]
