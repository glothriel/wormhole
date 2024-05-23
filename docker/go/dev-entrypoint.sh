#!/bin/sh

# /src should contain files bundled from dockerfile
# /src-tmp should have mounted volume, that will allow syncing file from Tilt

# if /src-tmp is empty, copy all files from /src to /src-tmp
if [ ! "$(ls -A /src-tmp)" ]; then
  cp -r /src/* /src-tmp
fi

# remove all files from /src-tmp, that are not in /src
cd /src-tmp
find . -type f -exec bash -c 'for file; do [ ! -e "/src/$file" ] && rm -f "$file"; done' bash {} +

args=$@
cwd=$(pwd)
echo "Starting watchexec on ${cwd}"

watchexec -n -q -r -e go,mod,sum -- sh -c "while true; do sleep 1 && go run main.go --debug ${args}; done"