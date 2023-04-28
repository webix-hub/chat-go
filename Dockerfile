FROM alpine:latest

WORKDIR /app
COPY ./chat /app
COPY ./demodata /app/demodata

CMD ["/app/chat"]