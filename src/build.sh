#!/bin/bash
VERSION=$(git rev-parse --short HEAD)
TIME=$(date)
echo $VERSION
echo $TIME
env GOOS=linux GOARCH=amd64 go build -ldflags "-X 'main._Version_=$VERSION' -X 'main._BuildTime_=$TIME'" -gcflags "-N -l " kubeinstall.go
