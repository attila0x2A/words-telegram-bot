#!/bin/sh -x
#
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

# This script builds and deploys bot to the server.
# The only argument that needs to be passed is the IP-address of the server.
# WORDS_TAG env variable can be used to specify tag that will be used to build image.

IP="$1"
WORDS_TAG="${WORDS_TAG:-words-dev}"
echo $WORDS_TAG

VOLUME="${WORDS_TAG}-vol"
IMAGE_PATH="/tmp/${WORDS_TAG}-image.tar"

sudo docker build --build-arg "IP=$IP" -t "$WORDS_TAG" .
sudo docker save "$WORDS_TAG" -o $IMAGE_PATH
sudo chmod +r $IMAGE_PATH
scp $IMAGE_PATH "root@$IP:/tmp/"
ssh "root@$IP" -- "
    docker load < $IMAGE_PATH
    docker volume create $VOLUME
"

set +x
echo
echo "You are about to stop an old version and start a new one. This can be disruptive. Are you sure?"
echo "Press enter to continue."
read _
set -x

APP="${WORDS_TAG}-app"

ssh "root@$IP" <<EOF
    docker stop $APP
    docker rm $APP
    docker run \
        --restart=on-failure \
        --name $APP \
        --mount source=${VOLUME},target=/words-vol/db/ \
        -p 8443:8443 \
        $WORDS_TAG --port 8443 --ip $IP
EOF
