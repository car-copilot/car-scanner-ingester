# OBD 2 InfluxDB

```shell
influx remote create --name store --remote-url http://0.0.0.0:8087 --remote-api-token y1ZPDzvF43o2WfEjwKAoEvPZhK_cj41PdloWbVhaabhUxyz3NFnxJCQjZqRwIfDS6rLGAp1YgcL2UbM1vvv7eg== --remote-org-id obicorp
```

```shell
influx replication create \
  --name store \
  --remote-id 0c7060b71a00c000 \
  --local-bucket-id ceb5911044307a80 \
  --remote-bucket audi_a4
```
