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

RUN apk update && apk add --no-cache git gcc g++ ca-certificates apache2-utils
WORKDIR /go/src/words
COPY *.go ./
Run go get -d -v -tags netgo -installsuffix netgo
# netgo and ldflags makes sure that dns resolver and binary are statically
# linked giving the ability for smaller images.
RUN go build -tags netgo -installsuffix netgo -ldflags '-extldflags "-static"' -o /go/bin/words

FROM scratch
# FIXME: Copying certificates looks a little bit hacky. Is there a better
# solution?
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/words /go/bin/words
COPY data/*.csv data/
# TODO: This should be moved to the CMD starting it up.
ENTRYPOINT ["/go/bin/words", "--db_path=/words-vol/db/db.sql"]
