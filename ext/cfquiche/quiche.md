# Adapter to Rust Quic implementation
```

git clone --recursive https://github.com/goburrow/quiche

docker build -t quiche:builder -f docker/Dockerfile docker/
docker run -i -t -v "$PWD:/usr/src/quiche" -u `id -u` -w "/usr/src/quiche" quiche:builder
$ mkdir out
$ export HOME=/usr/src/quiche/out
$ ./build.sh deps

ln -s /ws/dmesh-src/go-quiche/deps/quiche/target/release/libquiche.so /usr/local/lib/
```


