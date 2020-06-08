#!/usr/bin/env bash

echo $DOCKERHUB_TOKEN | docker login -u $DOCKERHUB_USERNAME --password-stdin;
export IMG="iter8/iter8-controller:$TRAVIS_BRANCH";
echo "Building PR Docker image - $IMG";
make docker-build;
make docker-push;
#LATEST="iter8/iter8-controller:latest";
#echo "Tagging image as latest - $LATEST";
#docker tag $IMG $LATEST;
#export IMG=$LATEST;
#make docker-push;
