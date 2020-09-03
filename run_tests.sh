#!/bin/sh
ginkgo build .
sudo ./vrf-cni.test
