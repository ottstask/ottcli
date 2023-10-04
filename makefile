
build:
	rm -rf tmp_build/scaffold tmp_build/websocket
	cp -rf ../../example/scaffold tmp_build/scaffold
	rm -rf tmp_build/scaffold/go.mod tmp_build/scaffold/daemon.log 
	go build -o gkit .
	rm -rf tmp_build/scaffold tmp_build/websocket
