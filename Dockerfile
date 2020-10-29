# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build and then copy over the neede parts to create a small image
FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git gcc g++ ca-certificates apache2-utils openssl
WORKDIR /go/src/words
RUN mkdir /ssl/
# FIXME: Use secret for the ip address instead of argument, so that it's not
# stored in the image?
# https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information
ARG IP
# FIXME: This probably should be part of the deployment automation and not
# dockerfile. If we were running this on multiple servers, docker image would
# need to be rebuild for each one, which isn't right.
RUN openssl req -newkey rsa:2048 -sha256 -nodes -keyout /ssl/webhook.key -x509\
        -days 365 -out /ssl/webhook.crt -subj "/CN=$IP"
COPY *.go ./
RUN go get -d -v -tags netgo -installsuffix netgo
# netgo and ldflags makes sure that dns resolver and binary are statically
# linked giving the ability for smaller images.
RUN go build -tags netgo -installsuffix netgo -ldflags '-extldflags "-static"' -o /go/bin/words

FROM scratch
# FIXME: Copying certificates looks a little bit hacky. Is there a better
# solution?
# These certificates are needed for http client to work with SSL.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# These certificates are needed for http server to work with SSL.
COPY --from=builder /ssl/webhook.key ssl/webhook.crt /ssl/
COPY --from=builder /go/bin/words /go/bin/words
# TODO: DB path (and maybe other args) should be moved to the CMD starting it up.
ENTRYPOINT ["/go/bin/words", "--db_path=/words-vol/db/db.sql", "--push", "--cert_path=/ssl/webhook.crt", "--key_path=/ssl/webhook.key"]
