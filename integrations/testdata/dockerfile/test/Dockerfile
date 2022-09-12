FROM alpine:3.16.1

RUN apk add --no-cache bash

COPY test/test.sh .

RUN chmod 755 test.sh

CMD [ "./test.sh" ]
