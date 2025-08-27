FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY bin/trading_assistant .

COPY web/build ./web/build

EXPOSE 8080

CMD ["./trading_assistant"]