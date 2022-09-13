FROM alpine:3.16.1

RUN apk add --no-cache bash jq curl

COPY test/test.sh .

RUN chmod 755 test.sh

CMD [ "./test.sh" ]
