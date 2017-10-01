run-server:
	go run ./server/server.go ./server/map.go ./server/copy.go 1234 4

run-dev:
	go run ./dev/dev.go

run-web:
	go run ./dev/web.go

run-cli:
	go run ./cli/cli.go git@github.com:wongnat/dummy.git cd46ed208331d82c36d5d2ed4e2818d388bf6796 ./cli

clean:
	rm -rf ./server/builds
	./scripts/clean.sh

test:
	# TODO

# TODO
build-container:
	docker build -t wongnat/amoeba .

# TODO
run-container:
	docker run -d -v /var/run/docker.sock:/var/run/docker.sock wongnat/amoeba
