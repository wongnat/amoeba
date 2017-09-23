FROM golang:1.8

RUN apt-get update
RUN apt-get install -y cmake
RUN apt-get install -y git
RUN apt-get install -y libgit2-dev
RUN apt-get install -y pkg-config
RUN apt-get install -y libseccomp-dev
RUN apt-get install -y btrfs-tools
RUN apt-get install -y libdevmapper-dev

WORKDIR /go/src/amoeba/
COPY . .

RUN go-wrapper download gopkg.in/yaml.v1

WORKDIR /go/src/amoeba/server/
CMD ["go-wrapper", "run"]

# TODO
#   mount host's docker.sock
#   mount this project directory
#   set DOCKER_HOST to be host machine
