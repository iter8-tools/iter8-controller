[![Build Status](https://travis-ci.com/iter8-tools/iter8.svg?branch=master)](https://travis-ci.com/iter8-tools/iter8)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

# iter8

> iter8 enables statistically robust continuous experimentation of microservices in your CI/CD pipelines.

For in-depth information about how to use iter8, visit [iter8.tools](https://iter8.tools).

## In this README:

- [Introduction](#introduction)
- [Repositories](#repositories)

## Introduction
Use an iter8 experiment to safely expose competing versions of a service to application traffic, gather in-depth insights about key performance and business metrics for your microservice versions, and intelligently rollout the best version of your service.

Iter8’s expressive model of cloud experimentation supports a variety of CI/CD scenarios. Using an iter8 experiment, you can:

1. Run a performance test with a single version of a microservice.
2. Perform a canary release with two versions, a baseline and a candidate. Iter8 will shift application traffic safely and gradually to the candidate, if it meets the criteria you specify in the experiment.
3. Perform an A/B test with two versions – a baseline and a candidate. Iter8 will identify and shift application traffic safely and gradually to the winner, where the winning version is defined by the criteria you specify in the experiment.
4. Perform an A/B/N test with multiple versions – a baseline and multiple candidates. Iter8 will identify and shift application traffic safely and gradually to the winner.

Under the hood, iter8 uses advanced Bayesian learning techniques coupled with multi-armed bandit approaches to compute a variety of statistical assessments for your microservice versions, and uses them to make robust traffic control and rollout decisions.

## Repositories

The components of iter8 are divided across a few github repositories.

- [iter8](https://github.com/iter8-tools/iter8) This repository containing the kubernetes controller that orchestrates iter8's experiments.
- [iter8-analytics](https://github.com/iter8-tools/iter8-analytics) The repository containing the iter8-analytics component.
- [iter8-trend](https://github.com/iter8-tools/iter8-trend) The repository contains the iter8-trend component.

In addition,
- iter8's extensions to Kiali is contained in [kiali](https://github.com/kiali/kiali), [kiali-ui](https://github.com/kiali/kiali-ui), and [k-charted](https://github.com/kiali/k-charted). 
- iter8's extensions to KUI is contained in [kui](https://github.com/IBM/kui).. 