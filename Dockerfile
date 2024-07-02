FROM alpine:3.15.11 AS builder

ARG POCKETVERSION=0.22.14

RUN apk add --no-cache \
  ca-certificates \
  unzip \
  curl 

RUN curl -L -o /tmp/pocketbase.zip https://github.com/pocketbase/pocketbase/releases/download/v${POCKETVERSION}/pocketbase_${POCKETVERSION}_linux_amd64.zip \
  && unzip /tmp/pocketbase.zip -d /tmp/pocketbase \
  && chmod +x /tmp/pocketbase/pocketbase

#############################################
# Final image
#############################################

FROM alpine:3.15.11
LABEL maintainer="Waldir Borba Junior <wborbajr@gmail.com>"

WORKDIR /app/pocketbase

COPY --from=builder /tmp/pocketbase .

ENV DATA_DIR=/app/data/pb_data
ENV PUBLIC_DIR=/app/data/pb_public
ENV PORT=8090

EXPOSE ${PORT}

CMD [ "sh", "-c", "./pocketbase serve --http=0.0.0.0:${PORT} --dir=${DATA_DIR} --publicDir=${PUBLIC_DIR}" ]
