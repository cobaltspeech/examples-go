#!groovy
// Copyright (2020) Cobalt Speech and Language Inc.
// This file is only needed for Cobalt's continuous integration system and
// is not required to build any of the examples.

// Keep only 10 builds on Jenkins
properties([
    buildDiscarder(logRotator(
        artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '', numToKeepStr: '10'))
])

// build using Cobalt's shared Jenkins library function
golangStdBuild()
