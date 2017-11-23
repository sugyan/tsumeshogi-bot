#!/bin/sh

if ! command -v gcloud > /dev/null; then
    echo 'Please install and enable `gcloud` command.'
    echo 'https://cloud.google.com/sdk/downloads'
    exit 1
fi

GOPATH=$(pwd)/gopath:$(PWD)/gopath/vendor gcloud app deploy app
