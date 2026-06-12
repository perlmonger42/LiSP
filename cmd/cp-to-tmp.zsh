#!/usr/bin/env zsh

GOMOD=$(go env GOMOD)
PROJECT_ROOT=${GOMOD:h}  # the root of the Go project (dir containing `go.mod`)
PROJECT_NAME=${PROJECT_ROOT:t}

SRC=$PROJECT_ROOT
DST=~/tmp  # .../$PROJECT_NAME will be created

echo SRC=$SRC
echo DST=$DST
rsync --archive             \
      --update              \
      --delete              \
      --verbose             \
      --hard-links          \
      --xattrs              \
      $SRC $DST
