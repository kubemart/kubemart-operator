#!/bin/bash

export INDEX_IMG=docker.io/kubemart/kubemart-olm-index:latest

docker pull $INDEX_IMG
INDEX_CONTAINER=$(docker create $INDEX_IMG)
docker start $INDEX_CONTAINER
docker cp $INDEX_CONTAINER:/database/index.db .

echo "Done. You can now open index.db file with DB Browser for SQLite (https://sqlitebrowser.org)."
open -a "DB Browser for SQLite" index.db # only works for Mac
