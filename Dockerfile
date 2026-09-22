FROM golang:1.24-bookworm AS build
ARG VERSION=0.1.0-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -tags="grammar_subset,grammar_subset_go,grammar_subset_javascript,grammar_subset_typescript,grammar_subset_tsx,grammar_subset_python,grammar_subset_java,grammar_subset_rust,grammar_subset_c_sharp,grammar_subset_php,grammar_subset_ruby" \
    -trimpath -ldflags="-s -w -X main.version=${VERSION#v}" -o /out/whichrepo ./cmd/whichrepo

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/whichrepo /usr/local/bin/whichrepo
ENTRYPOINT ["/usr/local/bin/whichrepo"]
