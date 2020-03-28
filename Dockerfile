FROM debian:jessie

COPY cmd/ms-operator.v1/ms-operator /ms-operator

ENTRYPOINT ["/ms-operator", "-alsologtostderr"]
