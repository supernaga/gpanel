FROM alpine:3.20
RUN apk add --no-cache curl jq
COPY run-agent.sh /usr/local/bin/run-agent.sh
RUN chmod +x /usr/local/bin/run-agent.sh
ENTRYPOINT ["/usr/local/bin/run-agent.sh"]
