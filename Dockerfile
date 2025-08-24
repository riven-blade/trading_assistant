FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY bin/trading_assistant .

COPY web/build ./web/build

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./trading_assistant"]