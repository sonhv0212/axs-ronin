# Build Geth in a stock Go builder container
FROM golang:1.22.10-alpine3.21@sha256:58a82e883f0d11f52204d2baa58a62dc565c409105773526340f678c9dc2558f AS builder

RUN apk add --no-cache make gcc musl-dev linux-headers git libstdc++-dev

COPY . /opt
RUN cd /opt && make ronin

# Pull Geth into a second stage deploy alpine container
FROM alpine:3.21@sha256:21dc6063fd678b478f57c0e13f47560d0ea4eeba26dfc947b2a4f81f686b9f45

RUN apk add --no-cache ca-certificates
WORKDIR "/opt"

ENV PASSWORD=''
ENV PRIVATE_KEY=''
ENV BOOTNODES=''
ENV VERBOSITY=3
ENV SYNC_MODE='snap'
ENV NETWORK_ID='2021'
ENV ETHSTATS_ENDPOINT=''
ENV NODEKEY=''
ENV FORCE_INIT='true'
ENV RONIN_PARAMS=''
ENV INIT_FORCE_OVERRIDE_CHAIN_CONFIG='false'
ENV ENABLE_FAST_FINALITY='true'
ENV ENABLE_FAST_FINALITY_SIGN='false'
ENV BLS_PRIVATE_KEY=''
ENV BLS_PASSWORD=''
ENV BLS_AUTO_GENERATE='false'
ENV BLS_SHOW_PRIVATE_KEY='false'
ENV GENERATE_BLS_PROOF='false'

COPY --from=builder /opt/build/bin/ronin /usr/local/bin/ronin
COPY --from=builder /opt/genesis/ ./
COPY --from=builder /opt/docker/chainnode/entrypoint.sh ./

EXPOSE 7000 6060 8545 8546 30303 30303/udp

ENTRYPOINT ["./entrypoint.sh"]
