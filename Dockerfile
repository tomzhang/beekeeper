FROM golang:1.14 AS build

WORKDIR /src
COPY . ./

RUN make binary

FROM debian:10.2-slim

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && apt-get install -y \
        ca-certificates; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*; \
    groupadd -r beekeeper --gid 999; \
    useradd -r -g beekeeper --uid 999 --no-log-init -m beekeeper;

COPY --from=build /src/dist/beekeeper /usr/local/bin/beekeeper

USER beekeeper
WORKDIR /home/beekeeper

ENTRYPOINT ["beekeeper"]