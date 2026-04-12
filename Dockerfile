# Build: docker build -t provider-kcore:dev .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /provider ./cmd/provider

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /provider /provider
USER nonroot:nonroot
ENTRYPOINT ["/provider"]
