# OBD 2 InfluxDB

Small tool to export Car Scanner data to InfluxDB

## Usage

```bash
Usage of car-scanner-ingester:
  -config string
        Path to the configuration file (default "config.yaml")
  -dry-run
        Dry run
  -list-pids
        List all PIDs
  -serve
        Start the HTTP server
```

Place CSV in `test/<owner>/<brand> - <model> - <engin> (<year>)`

## Configuration

An example configuration file is provided in `config.yaml`

You can overwrite configuration values with environment variables, by using the path to the configuration key in uppercase prefixed by `CSI_`:

```bash
export CSI_INFLUXDB_URL=http://localhost:8086
export CSI_INFLUXDB_TOKEN=my-token
export CSI_INFLUXDB_ORG=my-org
go run main.go -config config.yaml -dry-run=true
```
