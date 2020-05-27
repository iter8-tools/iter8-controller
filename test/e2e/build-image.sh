#!/usr/bin/env bash

# Only build images for master and v[0-9]+.[0-9]+ release branches
if [[ "$TRAVIS_BRANCH" == "master" ]] || [[ "$TRAVIS_BRANCH" =~ ^v[0-9]+\.[0-9]+$ ]] ; then
  echo $DOCKERHUB_TOKEN | docker login -u $DOCKERHUB_USERNAME --password-stdin;
  # Only build images when commits are done on these branches
  if [[ "$TRAVIS_PULL_REQUEST" == "false" ]]; then
    export IMG="iter8/iter8-controller:$TRAVIS_BUILD_NUMBER-$TRAVIS_COMMIT";
    echo "Building Docker image - $IMG";
    make docker-build;
    make docker-push;
    LATEST="iter8/iter8-controller:latest";
    echo "Tagging image as latest - $LATEST";
    docker tag $IMG $LATEST;
    export IMG=$LATEST;
    make docker-push;
  fi
fi

