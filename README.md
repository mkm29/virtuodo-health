# Virtuoso with Custom Health Check


```sh
TAG=agent-0.1.0
docker build -t virtuoso:$TAG .
docker run -p 1111:1111 -p 8890:8890 \
  --env DBA_PASSWORD=dba \
  -d virtuoso:latest
docker run -p 3333:3333 -d virtuoso:$TAG
```


## Extract status

```sh
STATUS=`isql localhost:1111 dba dba exec="status();"`
N=$(echo $STATUS | grep -oP '(\d{1,}) buffers' | cut -d' ' -f1)
USED=$(echo $STATUS | grep -oP ', (\d{1,}) used' | cut -d' ' -f2)
```

The `health` binary needs to extract and compute these metrics.
