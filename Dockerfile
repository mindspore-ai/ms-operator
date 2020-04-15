FROM ubuntu:18.04

COPY ms-operator /ms-operator

ENTRYPOINT ["/ms-operator", "-alsologtostderr"]
