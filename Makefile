run-server:
	go run ./server/server.go 1234 4

run-dev:
	go run ./dev/dev.go

run-cli:
	go run ./cli/main.go git@github.com:wongnat/dummy.git ed59cc75335f869d2378a79924332f17ca1beffa

clean:
	rm -rf ./out
	./scripts/clean.sh

test:
	# TODO

# TODO
build-container:
	docker build -t wongnat/amoeba .

# TODO
run-container:
	docker run -it --rm --name amoeba-test wongnat/amoeba
