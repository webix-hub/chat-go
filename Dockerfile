FROM centurylink/ca-certs
WORKDIR /app
COPY ./chat /app
COPY ./demodata /app/demodata

CMD ["/app/chat"]