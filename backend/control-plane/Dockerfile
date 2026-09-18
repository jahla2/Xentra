FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/xentra-control ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/xentra-control /xentra-control
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/xentra-control"]
