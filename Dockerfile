FROM golang:1.9

RUN apt-get update
RUN apt-get install -y cmake
RUN apt-get install -y git
RUN apt-get install -y libgit2-dev
RUN apt-get install -y pkg-config
RUN apt-get install -y libseccomp-dev
RUN apt-get install -y btrfs-tools
RUN apt-get install -y libdevmapper-dev

# TODO copy ssh key over

WORKDIR /go/src/amoeba/
COPY . .

RUN cd lib && go-wrapper download && go-wrapper install
RUN cd repo && go-wrapper download && go-wrapper install
RUN cd utils && go-wrapper download && go-wrapper install
RUN cd server && go-wrapper download && go-wrapper install

WORKDIR /go/src/amoeba/server/
CMD ["go-wrapper", "run", "./builds", "1234", "4"]
