#!/bin/sh -l

echo "PP here, $1"

time=$(date)

echo "::set-output name=time::$time"
