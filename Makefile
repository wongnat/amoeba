# # TODO
# build-container:
# 	docker build -t wongnat/amoeba .
#
# # TODO mount ssh key
# run-container:
# 	docker run -d -p 1234:1234 -v /var/run/docker.sock:/var/run/docker.sock wongnat/amoeba

build:
	mkdir build
	cd server && go build -o ../build/amoeba

run:
	./build/amoeba ./out 1234 4

# test:

clean:
	rm -rf build
	./scripts/clean.sh
