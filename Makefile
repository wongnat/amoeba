build:
	mkdir build
	cd server && go build -o ../build/amoeba

run:
	./build/amoeba ./build/out 1234 4

clean:
	rm -rf build
