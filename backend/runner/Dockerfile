FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/xentra-runner ./cmd/runner

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/xentra-runner /xentra-runner
USER nonroot:nonroot
EXPOSE 8090
ENTRYPOINT ["/xentra-runner"]
