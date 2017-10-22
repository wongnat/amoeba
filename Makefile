build:
	mkdir build
	cd server && go build -o ../build/amoeba

run:
	./build/amoeba ./build/out 1234 4

test:
	go run cli/cli.go git@github.com:wongnat/dummy.git cd46ed208331d82c36d5d2ed4e2818d388bf6796 ./build
clean:
	rm -rf build
	./scripts/clean.sh

# # TODO
# build-container:
# 	docker build -t wongnat/amoeba .
#
# # TODO mount ssh key
# run-container:
# 	docker run -d -p 1234:1234 -v /var/run/docker.sock:/var/run/docker.sock wongnat/amoeba
