run-server:
	go run ./server/server.go 1234 4

run-dev:
	go run ./dev/dev.go

clean:
	rm -rf ./out/*
	rm -rf ./groups/*
	./scripts/clean.sh

test:
	# TODO

# TODO
build-container:
	docker build -t wongnat/amoeba .

# TODO
run-container:
	docker run -it --rm --name amoeba-test wongnat/amoeba
