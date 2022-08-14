# builder image
FROM golang:alpine3.16 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o health .

# generate clean, final image for end users
FROM openlink/virtuoso-opensource-7:latest as prod
COPY --from=builder /build/health /usr/local/bin
RUN echo "status();" > status.sql
ENTRYPOINT [ "/bin/sh" ]
CMD [ "-c", "/usr/local/bin/health" ]