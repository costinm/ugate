
/usr/local/bin/envoy -c /var/lib/envoy.yaml

/usr/local/bin/ugate &

curl localhost:8081
while [ $? -ne 0 ]
do
  sleep 0.2
  curl localhost:8081
done
