#!/bin/sh

args=$@
watchexec -n -q -r -e go,mod,sum -- sh -c "while true; do sleep 1 && go run main.go --debug ${args}; done"