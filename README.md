# Amoeba

### Description
Amoeba is a web server that checks for breaking changes within a dockerized microservice architecture.
It integrates with github as a push/pull request event, so that when a new commit is pushed to a web service, Amoeba:
 1. Clones the commit of the web service.
 2. Builds the docker image of the new commit.
    * Build ID is in the form \<repo-name>-\<commit-sha>
 2. Clones the client repos of the web service.
 3. Runs the integration tests of clients of that web service via docker compose.
    * Serves the standard output of docker-compose invocations via websockets
 4. Reports if your commit caused any errors.

### Endpoints

Endpoint intended to be used as a github webhook url:
```
/build
```

Return a list of all builds currently running:
```
/build/ids
```

Return a list of all the client repos (as git ssh urls) of the specified build:
```
/build/{id}/clients
```

Websocket that serves the stdout of docker-compose in real-time:
```
/build/{id}/{client}/stdout
```

Note:
* {id}: \<repo-name>-\<commit-sha>
* {client}: git ssh url

### Dependencies
* [git2go](https://github.com/libgit2/git2go) - libgit2 wrapper for golang
  * Note: must install libgit2 on your system; follow the guide [here](https://libgit2.github.com/)
### Usage
To clone your repos from github, Amoeba depends on a valid ssh key existing in your home directory (i.e. $HOME/.ssh/id_rsa.pub).

```
make build
make run <port> <max-number-of-builds> # default is 8
```

### TODO
* Implement correct github push event response
* Implement tests
* Dockerize this project