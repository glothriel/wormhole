#!/bin/sh

watchexec -n -r -q -- go run main.go --debug listen --acceptor dummy
