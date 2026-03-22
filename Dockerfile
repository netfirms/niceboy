# Dockerfile for niceboy
# This Dockerfile is intended to be used with GoReleaser
FROM alpine:latest

# Install necessary packages for Go TUI and HTTPS requests
RUN apk add --no-cache ca-certificates tzdata ncurses

WORKDIR /app

# The binary is placed in the root of the build context by GoReleaser
COPY niceboy /usr/local/bin/niceboy
COPY config.example.yaml /app/config.yaml

# Set default environment
ENV TERM=xterm-256color

ENTRYPOINT ["niceboy"]
CMD ["-config", "config.yaml"]
